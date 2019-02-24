package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"strings"

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
}

func NewGmailHelper(srv *gm.Service, user string, conf *config.Config) *GmailHelper {

	accountHelper := api.NewAccountHelper(user, srv)
	msgHelper := api.NewMsgHelper(user, srv)
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

func (h *GmailHelper) PrintMessage(m *gm.Message) {
	var subject string
	var from string
	if m.Payload != nil && m.Payload.Headers != nil {
		for _, hdr := range m.Payload.Headers {
			if hdr.Name == "Subject" {
				subject = hdr.Value
			}
			if hdr.Name == "From" {
				from = api.GetFromName(hdr.Value)
			}
		}
	}
	if subject == "" {
		subject = "<No subject>"
	}
	if from == "" {
		from = "<unknown sender>"
	}

	labelNames := h.Msgs.MessageLabelNames(m)
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

func (h *GmailHelper) FilterMessagesByCategory(
	cat string, msgs []*gm.Message, categorizeThreads bool,
) ([]*gm.Message, error) {
	matchedMsgs := make([]*gm.Message, 0)
	detail := h.RequiredDetailForPlugins(cat)

	if categorizeThreads {
		// If any messages matches the category, then the whole thread does.
		msgsByThread := api.MessagesByThread(msgs)
		prnt.Hum.Always.P("Categorising threads ")
		progP := prnt.NewProgressPrinter(len(msgsByThread))
		for _, threadMsgs := range msgsByThread {
			progP.Progress(1)
			threadMatched := false
			for _, msg := range threadMsgs {
				msg, err := h.Msgs.GetMessage(msg.Id, detail)
				if err != nil {
					return nil, err
				}
				if h.MsgMatchesCategory(cat, msg) {
					threadMatched = true
					break
				}
			}

			if threadMatched {
				for _, msgId := range threadMsgs {
					msg, err := h.Msgs.GetMessage(msgId.Id, detail)
					if err != nil {
						return nil, err
					}
					matchedMsgs = append(matchedMsgs, msg)
				}
			}
		}
	} else {
		prnt.Hum.Always.P("Categorising messages ")
		progP := prnt.NewProgressPrinter(len(msgs))
		for _, msg := range msgs {
			progP.Progress(1)
			var err error
			msg, err = h.Msgs.GetMessage(msg.Id, detail)
			if err != nil {
				return nil, err
			}
			if h.MsgMatchesCategory(cat, msg) {
				matchedMsgs = append(matchedMsgs, msg)
			}
		}
	}

	prnt.Hum.Always.P("\n")
	return matchedMsgs, nil
}

func (h *GmailHelper) FilterMessagesByInterest(
	interest InterestLevel, msgs []*gm.Message) ([]*gm.Message, error) {

	switch interest {
	case Interesting:
		return h.FilterMessagesByCategory(plugin.CategoryInteresting, msgs, true)
	case Uninteresting:
		return h.FilterMessagesByCategory(plugin.CategoryUninteresting, msgs, true)
	case MaybeInteresting:
	}
	prnt.StderrLog.Fatalln("Cannot filter by MaybeInteresting")
	return nil, nil
}

func (h *GmailHelper) TouchMessages(msgs []*gm.Message) error {
	return h.Msgs.ApplyLabels(msgs, []string{h.conf.ApplyLabelOnTouch})
}

func (h *GmailHelper) GetPlugins() []*plugin.Plugin {
	if h.plugins == nil {
		h.plugins = plugin.LoadPlugins()
	}
	return h.plugins
}

func (h *GmailHelper) MsgMatchesCategory(cat string, m *gm.Message) bool {
	for _, plug := range h.GetPlugins() {
		if plug.MatchesCategory(cat, m, h.Msgs) {
			return true
		}
	}
	return false
}

type InterestLevel int

const (
	Uninteresting InterestLevel = iota
	MaybeInteresting
	Interesting
)

func (h *GmailHelper) RequiredDetailForPlugins(cat string) api.MessageDetailLevel {
	detail := api.LabelsOnly
	for _, plug := range h.GetPlugins() {
		detail = api.MoreDetailedLevel(
			detail,
			plug.DetailRequiredForCategory(cat))
	}
	return detail
}

func (h *GmailHelper) MsgInterestRequiredDetail() api.MessageDetailLevel {
	detail := h.RequiredDetailForPlugins(plugin.CategoryInteresting)
	detail = api.MoreDetailedLevel(
		detail,
		h.RequiredDetailForPlugins(plugin.CategoryUninteresting),
	)
	return detail
}

func (h *GmailHelper) MsgInterest(m *gm.Message) InterestLevel {
	if h.MsgMatchesCategory(plugin.CategoryInteresting, m) {
		return Interesting
	} else if h.MsgMatchesCategory(plugin.CategoryUninteresting, m) {
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
			strings.Join(h.Msgs.LabelNames(filter.Action.AddLabelIds), ", ")
	}
	if len(filter.Action.RemoveLabelIds) > 0 {
		actionsMap["RemoveLabelIds"] =
			strings.Join(h.Msgs.LabelNames(filter.Action.RemoveLabelIds), ", ")
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
