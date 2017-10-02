package cmd

import (
	"fmt"
	"io/ioutil"
	"log"
	"regexp"
	"strings"

	gm "google.golang.org/api/gmail/v1"
	"gopkg.in/yaml.v2"

	"github.com/tsiemens/gmail-tools/util"
)

const (
	ConfigYamlFileName = "config.yaml"

	caseIgnore = "(?i)"

	// Labels only
	messageFormatMinimal = "minimal"
	// Labels and payload data
	messageFormatMetadata = "metadata"
)

type Config struct {
	InterestingMessageQuery    string   `yaml:"InterestingMessageQuery"`
	UninterestingLabelPatterns []string `yaml:"UninterestingLabelPatterns"`
	InterestingLabelPatterns   []string `yaml:"InterestingLabelPatterns"`
	ApplyLabelToUninteresting  string   `yaml:"ApplyLabelToUninteresting"`

	uninterLabelRegexps []*regexp.Regexp
	interLabelRegexps   []*regexp.Regexp
}

func LoadConfig() *Config {
	confFname := util.RequiredHomeDirAndFile(util.UserAppDirName, ConfigYamlFileName)

	confData, err := ioutil.ReadFile(confFname)
	if err != nil {
		log.Fatalf("Failed to read file: %v", err)
	}

	conf := &Config{}
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
	return &GmailHelper{User: user, srv: srv, conf: conf}
}

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

func (h *GmailHelper) messageLabelNames(m *gm.Message) []string {
	h.requireLabels()
	var lNames []string
	for _, lId := range m.LabelIds {
		lNames = append(lNames, h.labels[lId])
	}
	return lNames
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
		labelsToShow = append(labelsToShow, l)
	}

	fmt.Printf("- %s [%s] %s\n", from, strings.Join(labelsToShow, ", "), subject)
}

func (h *GmailHelper) PrintMessagesByCategory(msgs []*gm.Message) {
	catNames := []string{"PERSONAL", "SOCIAL", "PROMOTIONS", "UPDATES", "FORUMS"}
	var catIds []string
	for _, cn := range catNames {
		catIds = append(catIds, "CATEGORY_"+cn)
	}
	msgsByCat := make(map[string][]*gm.Message)
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
			fmt.Println("Found no category for msg:")
			h.PrintMessage(m)
			log.Fatal()
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
}

type MessageDetailLevel int

const (
	IdsOnly MessageDetailLevel = iota
	LabelsOnly
	LabelsAndPayload
)

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

	util.Debugf("Querying messages: '%s'\n", fullQuery)
	r, err := h.srv.Users.Messages.List(h.User).Q(fullQuery).Do()
	if err != nil {
		return nil, fmt.Errorf("Unable to get messages: %v", err)
	}

	loadDetails := (detailLevel != IdsOnly)
	var format string
	switch detailLevel {
	case LabelsOnly:
		format = messageFormatMinimal
	case LabelsAndPayload:
		format = messageFormatMetadata
	}

	var msgs []*gm.Message
	for _, m := range r.Messages {
		var msg *gm.Message
		if loadDetails {
			msg, err = h.srv.Users.Messages.Get(h.User, m.Id).Format(format).Do()
			if err != nil {
				return nil, fmt.Errorf("Failed to get message: %v", err)
			}
		} else {
			msg = m
		}
		msgs = append(msgs, msg)
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
