package cmd

import (
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"

	gm "google.golang.org/api/gmail/v1"

	"github.com/tsiemens/gmail-tools/api"
	"github.com/tsiemens/gmail-tools/config"
	"github.com/tsiemens/gmail-tools/plugin"
	"github.com/tsiemens/gmail-tools/prnt"
	"github.com/tsiemens/gmail-tools/util"
)

type GmailHelper struct {
	User    string
	Account *api.AccountHelper
	Msgs    *api.MsgHelper

	srv  *gm.Service
	conf *config.Config

	plugins []*plugin.Plugin

	mutex sync.Mutex
}

func NewGmailHelper(srv *gm.Service, user string, conf *config.Config) *GmailHelper {

	accountHelper := api.NewAccountHelper(user, srv)
	msgHelper := api.NewMsgHelper(user, srv, UseCacheFile)
	helper := &GmailHelper{
		User: user, Account: accountHelper, Msgs: msgHelper,
		srv: srv, conf: conf}
	emailAddr, err := helper.Account.GetEmailAddress()
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

// ---------- Message methods ----------------

type MessageJson struct {
	MessageId string   `json:"messageId"`
	ThreadId  string   `json:"threadId"`
	Timestamp int64    `json:"timestamp"`
	Subject   string   `json:"subject"`
	From      string   `json:"from"`
	Labels    []string `json:"labels"`
}

func (h *GmailHelper) GetMessageJson(m *gm.Message) *MessageJson {
	msgJson := &MessageJson{}

	msgJson.MessageId = m.Id
	msgJson.ThreadId = m.ThreadId
	msgJson.Timestamp = m.InternalDate

	if m.Payload != nil && m.Payload.Headers != nil {
		for _, hdr := range m.Payload.Headers {
			if hdr.Name == "Subject" {
				msgJson.Subject = hdr.Value
			}
			if hdr.Name == "From" {
				msgJson.From = hdr.Value
			}
		}
	}

	labelNames := h.Msgs.MessageLabelNames(m)
	msgJson.Labels = append(msgJson.Labels, labelNames...)

	return msgJson
}

func (h *GmailHelper) PrintMessagesJson(msgs []*gm.Message) {
	var msgsJson []*MessageJson
	for _, msg := range msgs {
		msgsJson = append(msgsJson, h.GetMessageJson(msg))
	}

	bytes, err := json.MarshalIndent(msgsJson, "", "  ")
	if err != nil {
		fmt.Errorf("Failed to martial messages: %v", err)
		return
	}
	fmt.Printf("%s", string(bytes))
}

func (h *GmailHelper) PrintMessage(m *gm.Message, indent int) {
	var subject string
	var from string
	headers, err := api.GetMsgHeaders(m)
	if err != nil {
		headers = &api.Headers{}
	}
	if headers.Subject == "" {
		subject = "<No subject>"
	} else {
		subject = headers.Subject
	}
	if headers.From.Name == "" && headers.From.Address == "" {
		from = "<unknown sender>"
	} else if headers.From.Name != "" {
		from = headers.From.Name
	} else {
		from = headers.From.Address
	}

	labelNames := h.Msgs.MessageLabelNames(m)
	otherThreadLabels := []string{}

	threadId := m.ThreadId
	if h.Msgs.ThreadIsLoaded(threadId) {
		threadLabels, err := h.Msgs.ThreadLabelNames(threadId)
		if err != nil {
			otherThreadLabels = append(otherThreadLabels, "<ERROR LOADING THREAD LABELS>")
		} else {
			for _, l := range threadLabels {
				if !util.StringSliceContains(l, labelNames) {
					otherThreadLabels = append(otherThreadLabels, l)
				}
			}
		}
	}

	// Filter out some labels here
	var labelsToShow []string
	addLabelsToShow := func(labels []string, addThreadMarker bool) {
		for _, l := range labels {
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
					prnt.StderrLog.Fatalf("'%s' is not a valid color\n", color)
				}
			}

			if addThreadMarker {
				l = "(+ " + l + ")"
			}

			labelsToShow = append(labelsToShow, preColor+l+util.ResetC)
		}
	}

	addLabelsToShow(labelNames, false)
	addLabelsToShow(otherThreadLabels, true)

	maybeId := ""
	if util.DebugMode {
		maybeId = m.Id + " "
	}

	indentStr := strings.Repeat(" ", indent)

	fmt.Printf("%s%s- %s [%s] %s\n", indentStr, maybeId, from, strings.Join(labelsToShow, ", "), subject)
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
				h.PrintMessage(m, 0)
			}
		}
	}

	catNames = append(catNames, noCat)
	catIds = append(catIds, noCat)

	for i, cat := range catIds {
		catMsgs := msgsByCat[cat]
		if len(catMsgs) > 0 {
			fmt.Println(catNames[i])

			msgsByThread := make(map[string][]*gm.Message)
			for _, m := range catMsgs {
				var threadMsgs []*gm.Message
				var ok bool
				if threadMsgs, ok = msgsByThread[m.ThreadId]; ok {
					threadMsgs = append(threadMsgs, m)
				} else {
					threadMsgs = []*gm.Message{m}
				}
				msgsByThread[m.ThreadId] = threadMsgs
			}

			for threadId, msgs := range msgsByThread {
				if util.DebugMode {
					fmt.Println(threadId)
				}

				sort.Slice(
					msgs,
					func(i, j int) bool { return msgs[i].Id < msgs[j].Id })
				first := true
				for _, msg := range msgs {
					if first {
						h.PrintMessage(msg, 0)
						first = false
					} else {
						h.PrintMessage(msg, 3)
					}
				}
			}
		}
	}
}

func (h *GmailHelper) FilterMessages(
	msgs []*gm.Message, categorizeThreads bool,
	detail api.MessageDetailLevel,
	filter func(*gm.Message, *GmailHelper) bool,
) ([]*gm.Message, error) {
	matchedMsgs := make([]*gm.Message, 0)

	if categorizeThreads {
		concurrentQueries := api.MaxConcurrentRequests
		querySem := make(chan bool, concurrentQueries)

		msgChan := make(chan []*gm.Message, 100)
		errChan := make(chan error, 100)
		// If any messages matches the category, then the whole thread does.
		msgsByThread := api.MessageIdsByThread(msgs)
		prnt.Hum.Always.P("Categorising threads ")
		for _, threadMsgIds := range msgsByThread {
			go func(threadMsgs []*api.MessageId) {
				querySem <- true
				defer func() { <-querySem }()

				threadMatched := false
				for _, msg := range threadMsgs {
					msg, err := h.Msgs.GetMessage(msg.Id, detail)
					if err != nil {
						errChan <- err
						return
					}
					if filter(msg, h) {
						threadMatched = true
						break
					}
				}

				loadedThreadMsgs := make([]*gm.Message, 0, len(threadMsgs))
				if threadMatched {
					for _, msgId := range threadMsgs {
						msg, err := h.Msgs.GetMessage(msgId.Id, detail)
						if err != nil {
							errChan <- err
							return
						}
						loadedThreadMsgs = append(loadedThreadMsgs, msg)
					}
				}
				msgChan <- loadedThreadMsgs
			}(threadMsgIds)
		}

		errors := make([]error, 0)
		progP := prnt.NewProgressPrinter(len(msgsByThread))
		for i := 0; i < len(msgsByThread); i++ {
			progP.Progress(1)
			select {
			case loadedMsgs := <-msgChan:
				for _, msg := range loadedMsgs {
					matchedMsgs = append(matchedMsgs, msg)
				}
			case err := <-errChan:
				errors = append(errors, err)
			}
		}
		if len(errors) > 0 {
			return nil, errors[0]
		}
	} else {
		// TODO goroutine this
		prnt.Hum.Always.P("Categorising messages ")
		progP := prnt.NewProgressPrinter(len(msgs))
		for _, msg := range msgs {
			progP.Progress(1)
			var err error
			msg, err = h.Msgs.GetMessage(msg.Id, detail)
			if err != nil {
				return nil, err
			}
			if filter(msg, h) {
				matchedMsgs = append(matchedMsgs, msg)
			}
		}
	}

	prnt.Hum.Always.P("\n")
	return matchedMsgs, nil
}

func (h *GmailHelper) FilterMessagesByInterest(
	interest InterestCategory, msgs []*gm.Message) ([]*gm.Message, error) {

	detail := h.RequiredDetailForPluginInterest()

	filter := func(msg *gm.Message, h *GmailHelper) bool {
		i := h.MsgInterest(msg)
		return interest == i
	}

	return h.FilterMessages(msgs, true, detail, filter)
}

func (h *GmailHelper) FindOutdatedMessages(baseQuery string) []*gm.Message {

	outdatedMsgsSet := make(map[string]bool, 0)

	for _, plug := range h.GetPlugins() {
		if plug.OutdatedMessages != nil {
			prnt.Deb.Ln("Getting outdated messages from", plug.Name, "plugin.")
			pluginOutdated := plug.OutdatedMessages(baseQuery, h.Msgs)
			for _, m := range pluginOutdated {
				outdatedMsgsSet[m.Id] = true
			}
		}
	}

	outdatedMsgs := make([]*gm.Message, 0, len(outdatedMsgsSet))
	for id := range outdatedMsgsSet {
		msg, err := h.Msgs.GetMessage(id, api.IdsOnly)
		util.CheckErr(err)
		outdatedMsgs = append(outdatedMsgs, msg)
	}
	return outdatedMsgs
}

func (h *GmailHelper) TouchMessages(msgs []*gm.Message) error {
	return h.Msgs.ApplyLabels(msgs, []string{h.conf.ApplyLabelOnTouch})
}

func (h *GmailHelper) GetPlugins() []*plugin.Plugin {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	if h.plugins == nil {
		h.plugins = plugin.LoadPlugins()
	}
	return h.plugins
}

func (h *GmailHelper) MsgPlugInterest(m *gm.Message) plugin.InterestLevel {
	interest := plugin.UnknownInterest
	for _, plug := range h.GetPlugins() {
		interest = interest.Combine(plug.MessageInterest(m, h.Msgs))
	}
	return interest
}

type InterestCategory int

const (
	Uninteresting InterestCategory = iota
	MaybeInteresting
	Interesting
)

func (h *GmailHelper) RequiredDetailForPluginInterest() api.MessageDetailLevel {
	detail := api.LabelsOnly
	for _, plug := range h.GetPlugins() {
		detail = api.MoreDetailedLevel(
			detail,
			plug.DetailRequiredForInterest())
	}
	return detail
}

func (h *GmailHelper) MsgInterest(m *gm.Message) InterestCategory {
	i := h.MsgPlugInterest(m)
	if i == plugin.StronglyInteresting || i == plugin.WeaklyInteresting {
		return Interesting
	} else if i == plugin.StronglyUninteresting || i == plugin.WeaklyUninteresting {
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
	allCriteriaKeys := map[string]bool{}
	for k := range critMap {
		allCriteriaKeys[k] = true
	}
	if newCritMap != nil {
		for k := range newCritMap {
			allCriteriaKeys[k] = true
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

	for _, k := range util.SortStrSlice(util.StrBoolMapKeys(allCriteriaKeys)) {
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
			strings.Join(h.Msgs.LabelNames(filter.Action.AddLabelIds), ", ")
	}
	if len(filter.Action.RemoveLabelIds) > 0 {
		actionsMap["RemoveLabelIds"] =
			strings.Join(h.Msgs.LabelNames(filter.Action.RemoveLabelIds), ", ")
	}
	if filter.Action.Forward != "" {
		actionsMap["Forward"] = filter.Action.Forward
	}

	for _, k := range util.SortStrSlice(util.StrStrMapKeys(actionsMap)) {
		prnt.LPrintln(prntT, fmt.Sprintf("  -> %s: %s", k, actionsMap[k]))
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
