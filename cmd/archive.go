package cmd

import (
	"fmt"
	"io/ioutil"
	"log"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
	gm "google.golang.org/api/gmail/v1"
	"gopkg.in/yaml.v2"

	"github.com/tsiemens/gmail-tools/api"
	"github.com/tsiemens/gmail-tools/util"
)

const (
	ConfigYamlFileName = "config.yaml"

	caseIgnore   = "(?i)"
	inboxLabelId = "INBOX"
)

var fromFieldRegexp = regexp.MustCompile(`\s*(\S|\S.*\S)\s*<.*>\s*`)

var dry = false
var verbose = false
var archiveRead = false

func messageLabelNames(m *gm.Message, labels map[string]string) []string {
	var lNames []string
	for _, lId := range m.LabelIds {
		lNames = append(lNames, labels[lId])
	}
	return lNames
}

func printMessage(m *gm.Message, labels map[string]string) {
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

	labelNames := messageLabelNames(m, labels)
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

func printMessagesByCategory(msgs []*gm.Message, labels map[string]string) {
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
			printMessage(m, labels)
			log.Fatal()
		}
	}

	for i, cat := range catIds {
		catMsgs := msgsByCat[cat]
		if len(catMsgs) > 0 {
			fmt.Println(catNames[i])
			for _, m := range catMsgs {
				printMessage(m, labels)
			}
		}
	}
}

type ConfigModel struct {
	DoNotArchiveQuery         string   `yaml:"DoNotArchiveQuery"`
	ArchiveLabelPatterns      []string `yaml:"ArchiveLabelPatterns"`
	DoNotArchiveLabelPatterns []string `yaml:"DoNotArchiveLabelPatterns"`
	ApplyExtraArchiveLabel    string   `yaml:"ApplyExtraArchiveLabel"`

	archiveLabelRegexps []*regexp.Regexp
	dnaLabelRegexps     []*regexp.Regexp
}

func loadConfig() *ConfigModel {
	confFname := util.RequiredHomeDirAndFile(util.UserAppDirName, ConfigYamlFileName)

	confData, err := ioutil.ReadFile(confFname)
	if err != nil {
		log.Fatalf("Failed to read file: %v", err)
	}

	conf := &ConfigModel{}
	err = yaml.Unmarshal(confData, conf)
	if err != nil {
		log.Fatalf("Could not unmarshal: %v", err)
	}
	util.Debugf("config: %+v\n", conf)

	for _, pat := range conf.ArchiveLabelPatterns {
		re, err := regexp.Compile(caseIgnore + pat)
		if err != nil {
			break
		}
		conf.archiveLabelRegexps = append(conf.archiveLabelRegexps, re)
	}
	if err == nil {
		for _, pat := range conf.DoNotArchiveLabelPatterns {
			re, err := regexp.Compile(caseIgnore + pat)
			if err != nil {
				break
			}
			conf.dnaLabelRegexps = append(conf.dnaLabelRegexps, re)
		}
	}
	if err != nil {
		log.Fatalf("Failed to load config: \"%s\"", err)
	}
	return conf
}

func getLabels(srv *gm.Service) map[string]string {
	r, err := srv.Users.Labels.List(api.DefaultUser).Do()
	if err != nil {
		log.Fatalf("Unable to retrieve labels. %v", err)
	}
	labelMap := make(map[string]string)
	for _, l := range r.Labels {
		labelMap[l.Id] = l.Name
	}
	return labelMap
}

type Archiver struct {
	srv    *gm.Service
	labels map[string]string
	conf   *ConfigModel
}

func NewArchiver(srv *gm.Service, labels map[string]string, conf *ConfigModel) *Archiver {
	if srv == nil || labels == nil || conf == nil {
		log.Fatalln("Internal error creating Archiver: ", srv, labels, conf)
	}
	return &Archiver{srv: srv, labels: labels, conf: conf}
}

func (a *Archiver) LoadMsgsToArchive() []*gm.Message {
	var unreadOnly string
	if !archiveRead {
		unreadOnly = "label:unread"
	}

	r, err := a.srv.Users.Messages.List(api.DefaultUser).
		Q("in:inbox " + unreadOnly + " -(" + a.conf.DoNotArchiveQuery + ")").Do()
	if err != nil {
		log.Fatalf("Unable to retrieve messages: %v", err)
	}

	var msgsToArchive []*gm.Message
	for _, m := range r.Messages {
		msg, err := a.srv.Users.Messages.Get(api.DefaultUser, m.Id).Do()
		if err != nil {
			log.Fatal("Failed to get message")
		}
		if a.ShouldArchive(msg) {
			msgsToArchive = append(msgsToArchive, msg)
		}
	}
	return msgsToArchive
}

func (a *Archiver) ShouldArchive(m *gm.Message) bool {
	var matchedIgnored = false
	var matchedDni = false

	for _, lId := range m.LabelIds {
		lName := a.labels[lId]
		labelIgnored := false
		for _, labRe := range a.conf.archiveLabelRegexps {
			idxSlice := labRe.FindStringIndex(lName)
			if idxSlice != nil {
				labelIgnored = true
				break
			}
		}
		matchedIgnored = matchedIgnored || labelIgnored
		if labelIgnored {
			// If we have ignored the label, then the do-not-ignore label
			// patterns are not applied.
			continue
		}
		for _, labRe := range a.conf.dnaLabelRegexps {
			idxSlice := labRe.FindStringIndex(lName)
			if idxSlice != nil {
				matchedDni = true
				break
			}
		}
		if matchedDni {
			break
		}
	}
	return matchedIgnored && !matchedDni
}

func (a *Archiver) LabelIdFromName(label string) string {
	for lId, lName := range a.labels {
		if label == lName {
			return lId
		}
	}
	log.Fatalf("No label named %s found\n", label)
	return ""
}

func (a *Archiver) ArchiveMessages(msgs []*gm.Message) error {
	var addLabels []string

	var extraLabelId string
	if a.conf.ApplyExtraArchiveLabel != "" {
		extraLabelId = a.LabelIdFromName(a.conf.ApplyExtraArchiveLabel)
		addLabels = append(addLabels, extraLabelId)
	}

	var msgIds []string

	for _, msg := range msgs {
		msgIds = append(msgIds, msg.Id)
	}
	modReq := gm.BatchModifyMessagesRequest{
		AddLabelIds:    addLabels,
		RemoveLabelIds: []string{inboxLabelId},
		Ids:            msgIds,
	}

	return a.srv.Users.Messages.BatchModify(api.DefaultUser, &modReq).Do()
}

func runArchiveCmd(cmd *cobra.Command, args []string) {
	conf := loadConfig()

	srv := api.NewGmailClient(api.ModifyScope)

	fmt.Print("Fetching inbox... ")
	labels := getLabels(srv)

	arch := NewArchiver(srv, labels, conf)
	msgsToArchive := arch.LoadMsgsToArchive()
	fmt.Print("done\n")

	if len(msgsToArchive) > 0 {
		if verbose {
			fmt.Println("Messages to archive:")
			printMessagesByCategory(msgsToArchive, labels)
			fmt.Print("\n")
		}

		fmt.Printf("Message count: %d\n", len(msgsToArchive))
		if conf.ApplyExtraArchiveLabel != "" {
			fmt.Printf("Extra label %s will be applied.\n", conf.ApplyExtraArchiveLabel)
		}

		if dry {
			fmt.Println("Skipping committing changes (--dry provided)")
		} else {
			if util.ConfirmFromInput("Archive these?") {
				err := arch.ArchiveMessages(msgsToArchive)
				if err != nil {
					log.Fatalf("Failed to archive messages: %s\n", err)
				} else {
					fmt.Println("Messages archived")
				}
			}
		}
	} else {
		fmt.Println("No messages to archive")
	}
}

// archiveCmd represents the archive command
var archiveCmd = &cobra.Command{
	Use: "archive",
	Short: "Attempts to archive unread messages in the inbox, based on the rules " +
		"in ~/.gmailcli/config.yaml",
	Run: runArchiveCmd,
}

func init() {
	RootCmd.AddCommand(archiveCmd)

	archiveCmd.Flags().BoolVarP(&dry, "dry", "n", false,
		"Perform no action, just print what would be done")
	archiveCmd.Flags().BoolVarP(&verbose, "verbose", "v", false,
		"Print verbose output (show all messages to archive)")
	archiveCmd.Flags().BoolVarP(&archiveRead, "include-read", "r", false,
		"Archive read and unread inbox messages")
}
