package cmd

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	gm "google.golang.org/api/gmail/v1"
	"gopkg.in/yaml.v2"

	"github.com/tsiemens/gmail-tools/prnt"
	"github.com/tsiemens/gmail-tools/util"
)

const (
	ConfigYamlFileName = "config.yaml"

	caseIgnore = "(?i)"
)

// Format values
const (
	// Default
	messageFormatFull = "full"
	// Labels only
	messageFormatMinimal = "minimal"
	// Labels and payload data
	messageFormatMetadata = "metadata"
	messageFormatRaw      = "raw"
)

type Config struct {
	InterestingMessageQuery    string            `yaml:"InterestingMessageQuery"`
	UninterestingLabelPatterns []string          `yaml:"UninterestingLabelPatterns"`
	InterestingLabelPatterns   []string          `yaml:"InterestingLabelPatterns"`
	ApplyLabelToUninteresting  string            `yaml:"ApplyLabelToUninteresting"`
	ApplyLabelOnTouch          string            `yaml:"ApplyLabelOnTouch"`
	LabelColors                map[string]string `yaml:"LabelColors"`

	uninterLabelRegexps []*regexp.Regexp
	interLabelRegexps   []*regexp.Regexp
	configFile          string
}

func LoadConfig() *Config {
	confFname := util.RequiredHomeDirAndFile(util.UserAppDirName, ConfigYamlFileName)

	confData, err := ioutil.ReadFile(confFname)
	if err != nil {
		log.Fatalf("Failed to read file: %v", err)
	}

	conf := &Config{}
	conf.configFile = confFname
	err = yaml.Unmarshal(confData, conf)
	if err != nil {
		log.Fatalf("Could not unmarshal: %v", err)
	}
	util.Debugf("config: %+v\n", conf)

	for _, pat := range conf.UninterestingLabelPatterns {
		re, err := regexp.Compile(caseIgnore + pat)
		if err != nil {
			break
		}
		conf.uninterLabelRegexps = append(conf.uninterLabelRegexps, re)
	}
	if err == nil {
		for _, pat := range conf.InterestingLabelPatterns {
			re, err := regexp.Compile(caseIgnore + pat)
			if err != nil {
				break
			}
			conf.interLabelRegexps = append(conf.interLabelRegexps, re)
		}
	}
	if err != nil {
		log.Fatalf("Failed to load config: \"%s\"", err)
	}
	return conf
}

type GmailHelper struct {
	User string

	srv    *gm.Service
	labels map[string]string // Label ID to label name
	conf   *Config
}

func NewGmailHelper(srv *gm.Service, user string, conf *Config) *GmailHelper {
	helper := &GmailHelper{User: user, srv: srv, conf: conf}
	emailAddr, err := helper.GetEmailAddress()
	if err != nil {
		prnt.StderrLog.Fatalln("Failed to get account email", err)
	}
	prnt.LPrintln(prnt.Verbose, "Account email:", emailAddr)
	// If the user has provided the --assert-email option, perform that check.
	if EmailToAssert != "" && emailAddr != EmailToAssert {
		prnt.StderrLog.Fatalf("Authorized account for %s did not match %s\n",
			emailAddr, EmailToAssert)
	}
	return helper
}

func (h *GmailHelper) GetEmailAddress() (string, error) {
	r, err := h.srv.Users.GetProfile(h.User).Do()
	if err != nil {
		return "", err
	}
	return r.EmailAddress, nil
}

// ---------- Message methods ----------------

func (h *GmailHelper) loadLabels() error {
	util.Debugln("Loading labels")
	r, err := h.srv.Users.Labels.List(h.User).Do()
	if err != nil {
		return err
	}
	labelMap := make(map[string]string)
	for _, l := range r.Labels {
		labelMap[l.Id] = l.Name
	}
	h.labels = labelMap
	return nil
}

func (h *GmailHelper) requireLabels() {
	if h.labels == nil {
		err := h.loadLabels()
		if err != nil {
			log.Fatalf("Failed to load labels: %v\n", err)
		}
	}
}

func (h *GmailHelper) LabelName(lblId string) string {
	h.requireLabels()
	return h.labels[lblId]
}

func (h *GmailHelper) labelNames(ids []string) []string {
	h.requireLabels()
	var lNames []string
	for _, lId := range ids {
		lNames = append(lNames, h.labels[lId])
	}
	return lNames
}

func (h *GmailHelper) messageLabelNames(m *gm.Message) []string {
	return h.labelNames(m.LabelIds)
}

var fromFieldRegexp = regexp.MustCompile(`\s*(\S|\S.*\S)\s*<.*>\s*`)

func (h *GmailHelper) PrintMessage(m *gm.Message) {
	var subject string
	var from string
	if m.Payload != nil && m.Payload.Headers != nil {
		for _, hdr := range m.Payload.Headers {
			if hdr.Name == "Subject" {
				subject = hdr.Value
			}
			if hdr.Name == "From" {
				matches := fromFieldRegexp.FindStringSubmatch(hdr.Value)
				if len(matches) > 0 {
					from = matches[1]
				} else {
					from = hdr.Value
				}
			}
		}
	}
	if subject == "" {
		subject = "<No subject>"
	}
	if from == "" {
		from = "<unknown sender>"
	}

	labelNames := h.messageLabelNames(m)
	// Filter out some labels here
	var labelsToShow []string
	for _, l := range labelNames {
		if !util.DebugMode &&
			(strings.HasPrefix(l, "CATEGORY_") ||
				l == "INBOX") {
			continue
		}
		preColor := ""
		if color, ok := h.conf.LabelColors[l]; ok {
			colorCode, ok := util.Colors[color]
			if ok {
				preColor = colorCode + util.Bold
			} else {
				fmt.Printf("'%s' is not a valid color\n", color)
				os.Exit(1)
			}
		}
		labelsToShow = append(labelsToShow, preColor+l+util.ResetC)
	}

	fmt.Printf("- %s [%s] %s\n", from, strings.Join(labelsToShow, ", "), subject)
}

func (h *GmailHelper) PrintMessagesByCategory(msgs []*gm.Message) {
	noCat := "NO CATEGORY"
	catNames := []string{"PERSONAL", "SOCIAL", "PROMOTIONS", "UPDATES", "FORUMS"}
	var catIds []string
	for _, cn := range catNames {
		catIds = append(catIds, "CATEGORY_"+cn)
	}
	msgsByCat := make(map[string][]*gm.Message)
	msgsByCat[noCat] = make([]*gm.Message, 0)
	for _, id := range catIds {
		msgsByCat[id] = make([]*gm.Message, 0)
	}

	for _, m := range msgs {
		foundCat := false
		for _, lId := range m.LabelIds {
			if _, ok := msgsByCat[lId]; ok {
				msgsByCat[lId] = append(msgsByCat[lId], m)
				foundCat = true
			}
		}
		if !foundCat {
			msgsByCat[noCat] = append(msgsByCat[noCat], m)
			util.Debugf("Found no category for msg. Had labels %+v\n", m.LabelIds)
			if util.DebugMode {
				h.PrintMessage(m)
			}
		}
	}

	for i, cat := range catIds {
		catMsgs := msgsByCat[cat]
		if len(catMsgs) > 0 {
			fmt.Println(catNames[i])
			for _, m := range catMsgs {
				h.PrintMessage(m)
			}
		}
	}
	noCatMsgs := msgsByCat[noCat]
	if len(noCatMsgs) > 0 {
		fmt.Println(noCat)
		for _, m := range noCatMsgs {
			h.PrintMessage(m)
		}
	}

}

type MessageDetailLevel int

const (
	IdsOnly MessageDetailLevel = iota
	LabelsOnly
	LabelsAndPayload
)

func (h *GmailHelper) LoadDetailedMessages(msgs []*gm.Message,
	detailLevel MessageDetailLevel) ([]*gm.Message, error) {

	var format string
	switch detailLevel {
	case IdsOnly:
		log.Fatalln("Invalid detailLevel: IdsOnly")
	case LabelsOnly:
		format = messageFormatMinimal
	case LabelsAndPayload:
		format = messageFormatMetadata
	}

	var detailedMsgs []*gm.Message

	printLvl := prnt.Always
	prnt.HPrint(printLvl, "Loading message details ")
	for i, msg := range msgs {
		progressStr := fmt.Sprintf("%d/%d", i+1, len(msgs))
		prnt.HPrint(printLvl, progressStr)

		dMsg, err := h.srv.Users.Messages.Get(h.User, msg.Id).Format(format).Do()
		if err != nil {
			return nil, fmt.Errorf("Failed to get message: %v", err)
		}
		detailedMsgs = append(detailedMsgs, dMsg)
		prnt.HPrint(printLvl, strings.Repeat("\x08", len(progressStr)))
	}
	prnt.HPrint(printLvl, "\n")

	return detailedMsgs, nil
}

func (h *GmailHelper) LoadMessage(id string) (*gm.Message, error) {
	return h.srv.Users.Messages.Get(h.User, id).Do()
}

func (h *GmailHelper) QueryMessages(query string, inboxOnly bool, unreadOnly bool,
	detailLevel MessageDetailLevel) ([]*gm.Message, error) {

	fullQuery := ""
	if inboxOnly {
		fullQuery += "in:inbox "
	}
	if unreadOnly {
		fullQuery += "label:unread "
	}

	fullQuery += query

	pageToken := ""
	queriedPageCnt := 0
	var msgs []*gm.Message

	for queriedPageCnt == 0 || pageToken != "" {
		queriedPageCnt++
		util.Debugf("Querying messages: '%s', page: %d\n", fullQuery, queriedPageCnt)

		call := h.srv.Users.Messages.List(h.User).Q(fullQuery)
		if pageToken != "" {
			call = call.PageToken(pageToken)
		}
		r, err := call.Do()
		if err != nil {
			return nil, fmt.Errorf("Unable to get messages: %v", err)
		}

		pageToken = r.NextPageToken

		for _, m := range r.Messages {
			msgs = append(msgs, m)
		}
	}

	if detailLevel != IdsOnly {
		var err error
		msgs, err = h.LoadDetailedMessages(msgs, detailLevel)
		if err != nil {
			return nil, err
		}
	}
	return msgs, nil
}

func (h *GmailHelper) LabelIdFromName(label string) string {
	h.requireLabels()
	for lId, lName := range h.labels {
		if label == lName {
			return lId
		}
	}
	log.Fatalf("No label named %s found\n", label)
	return ""
}

func (h *GmailHelper) BatchModifyMessages(msgs []*gm.Message,
	modReq *gm.BatchModifyMessagesRequest) error {

	var msgIds []string
	for _, msg := range msgs {
		msgIds = append(msgIds, msg.Id)
	}

	modReq.Ids = msgIds
	return h.srv.Users.Messages.BatchModify(h.User, modReq).Do()
}

func (h *GmailHelper) TouchMessages(msgs []*gm.Message) error {
	touchLabelId := h.LabelIdFromName(h.conf.ApplyLabelOnTouch)

	modReq := gm.BatchModifyMessagesRequest{
		AddLabelIds: []string{touchLabelId},
	}
	return h.BatchModifyMessages(msgs, &modReq)
}

type InterestLevel int

const (
	Uninteresting InterestLevel = iota
	MaybeInteresting
	Interesting
)

func (h *GmailHelper) MsgInterest(m *gm.Message) InterestLevel {
	var matchedUninter = false
	var matchedInter = false

	for _, lId := range m.LabelIds {
		lName := h.LabelName(lId)
		labelIsUninteresting := false
		for _, labRe := range h.conf.uninterLabelRegexps {
			idxSlice := labRe.FindStringIndex(lName)
			if idxSlice != nil {
				labelIsUninteresting = true
				break
			}
		}
		matchedUninter = matchedUninter || labelIsUninteresting
		if labelIsUninteresting {
			// If the label is explicitly uninteresting, then the "interesting" label
			// patterns are not applied.
			continue
		}
		for _, labRe := range h.conf.interLabelRegexps {
			idxSlice := labRe.FindStringIndex(lName)
			if idxSlice != nil {
				matchedInter = true
				break
			}
		}
		if matchedInter {
			break
		}
	}
	if matchedInter {
		return Interesting
	} else if matchedUninter {
		return Uninteresting
	}
	return MaybeInteresting
}

// ---------- Filter methods ----------------

// Initialized in init()
var CriteriaAttrs []string

func GetFieldAttr(x interface{}, attrName string) interface{} {
	// x must be a Ptr or interface
	v := reflect.ValueOf(x).Elem()
	return v.FieldByName(attrName).Interface()
}

func GetFieldAttrString(x interface{}, attrName string) string {
	val := GetFieldAttr(x, attrName)
	return fmt.Sprintf("%v", val)
}

func SetFieldAttr(x interface{}, attrName string, val interface{}) {
	// x must be a Ptr or interface
	v := reflect.ValueOf(x).Elem()
	v.FieldByName(attrName).Set(reflect.ValueOf(val))
}

func SetFieldAttrFromString(x interface{}, attrName string, valStr string) error {
	fieldAttr := GetFieldAttr(x, attrName)
	switch fieldAttr.(type) {
	case string:
		SetFieldAttr(x, attrName, valStr)
	case bool:
		b, err := strconv.ParseBool(valStr)
		if err != nil {
			return err
		}
		SetFieldAttr(x, attrName, b)
	case int64:
		i, err := strconv.ParseInt(valStr, 10, 64)
		if err != nil {
			return err
		}
		SetFieldAttr(x, attrName, i)
	default:
		util.Assert(false, "Unsupported type for attr:", attrName)
	}
	return nil
}

func (h *GmailHelper) GetFilters() ([]*gm.Filter, error) {
	r, err := h.srv.Users.Settings.Filters.List(h.User).Do()
	if err != nil {
		return nil, err
	}
	return r.Filter, nil
}

func (h *GmailHelper) MatchesFilter(regex *regexp.Regexp, filter *gm.Filter) bool {
	criteriaStr := fmt.Sprintf("%+v", filter.Criteria)
	return regex.MatchString(criteriaStr)
}

func (h *GmailHelper) printFilterAndMaybeDiff(filter, newFilter *gm.Filter) {
	isDiff := (newFilter != nil)
	prntT := prnt.Always
	if isDiff {
		prntT = prnt.Quietable
	}

	prnt.LPrintf(prntT, "Filter %s\n", filter.Id)
	defaultCrit := &gm.FilterCriteria{}

	getCriteriaMap := func(criteria *gm.FilterCriteria) map[string]string {
		cm := map[string]string{}
		for _, critAttr := range CriteriaAttrs {
			attrValStr := GetFieldAttrString(criteria, critAttr)
			if GetFieldAttrString(defaultCrit, critAttr) != attrValStr {
				cm[critAttr] = attrValStr
			}
		}
		return cm
	}

	critMap := getCriteriaMap(filter.Criteria)
	var newCritMap map[string]string
	if newFilter != nil {
		newCritMap = getCriteriaMap(newFilter.Criteria)
	}

	// Make a set of all keys that need to be shown
	allCriteriaKeys := map[string]byte{}
	for k, _ := range critMap {
		allCriteriaKeys[k] = 255
	}
	if newCritMap != nil {
		for k, _ := range newCritMap {
			allCriteriaKeys[k] = 255
		}
	}

	getValDisplayStr := func(critMap map[string]string, critName string) string {
		var valStr string
		if v, ok := critMap[critName]; ok {
			valStr = v
		} else {
			valStr = GetFieldAttrString(defaultCrit, critName)
			if valStr == "" {
				valStr = "<None>"
			}
		}
		return valStr
	}

	for k, _ := range allCriteriaKeys {
		oldValStr := getValDisplayStr(critMap, k)
		oldValLine := fmt.Sprintf("  %s: %s", k, oldValStr)
		var newValLine string

		if isDiff {
			newValStr := getValDisplayStr(newCritMap, k)
			if newValStr != oldValStr {
				// Print the lines as a diff
				oldValLine = prnt.Colorize("-"+oldValLine, "red")
				newValLine = prnt.Colorize(fmt.Sprintf("+  %s: %s", k, newValStr),
					"green")
			}
		}

		prnt.LPrintln(prntT, oldValLine)
		if newValLine != "" {
			prnt.LPrintln(prntT, newValLine)
		}
	}

	// Print all actions to apply the filter
	actionsMap := map[string]string{}
	if len(filter.Action.AddLabelIds) > 0 {
		actionsMap["AddLabelIds"] =
			strings.Join(h.labelNames(filter.Action.AddLabelIds), ", ")
	}
	if len(filter.Action.RemoveLabelIds) > 0 {
		actionsMap["RemoveLabelIds"] =
			strings.Join(h.labelNames(filter.Action.RemoveLabelIds), ", ")
	}
	if filter.Action.Forward != "" {
		actionsMap["Forward"] = filter.Action.Forward
	}

	for k, v := range actionsMap {
		prnt.LPrintln(prntT, fmt.Sprintf("  -> %s: %s", k, v))
	}
}

func (h *GmailHelper) PrintFilter(filter *gm.Filter) {
	h.printFilterAndMaybeDiff(filter, nil)
}

func (h *GmailHelper) PrintFilterDiff(oldFilter, newFilter *gm.Filter) {
	h.printFilterAndMaybeDiff(oldFilter, newFilter)
}

func (h *GmailHelper) CreateFilter(filter *gm.Filter) (*gm.Filter, error) {
	return h.srv.Users.Settings.Filters.Create(h.User, filter).Do()
}

func (h *GmailHelper) DeleteFilter(id string) error {
	return h.srv.Users.Settings.Filters.Delete(h.User, id).Do()
}

func init() {
	crit := gm.FilterCriteria{}
	critV := reflect.ValueOf(crit)
	critType := reflect.ValueOf(crit).Type()

	CriteriaAttrs = make([]string, 0, critV.NumField()-2)
	for i := 0; i < critV.NumField(); i++ {
		switch critV.Field(i).Interface().(type) {
		case []string:
			// Ignore these attrs. ForceSendFields and NullFields
		default:
			CriteriaAttrs = append(CriteriaAttrs, critType.Field(i).Name)
		}
	}
}
