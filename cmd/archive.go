package cmd

import (
	"fmt"
	"io/ioutil"
	"log"
	"regexp"

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

var dry = false
var verbose = false
var archiveRead = false

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

type Archiver struct {
	srv    *gm.Service
	conf   *ConfigModel
	helper *GmailHelper
}

func NewArchiver(srv *gm.Service, conf *ConfigModel, helper *GmailHelper) *Archiver {
	if srv == nil || conf == nil || helper == nil {
		log.Fatalln("Internal error creating Archiver: ", srv, conf, helper)
	}
	return &Archiver{srv: srv, conf: conf, helper: helper}
}

func (a *Archiver) LoadMsgsToArchive() []*gm.Message {
	msgs, err := a.helper.QueryMessages(" -("+a.conf.DoNotArchiveQuery+")",
		true, !archiveRead, true)
	if err != nil {
		log.Fatalf("%v\n", err)
	}

	var msgsToArchive []*gm.Message
	for _, msg := range msgs {
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
		lName := a.helper.LabelName(lId)
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

func (a *Archiver) ArchiveMessages(msgs []*gm.Message) error {
	var addLabels []string

	var extraLabelId string
	if a.conf.ApplyExtraArchiveLabel != "" {
		extraLabelId = a.helper.LabelIdFromName(a.conf.ApplyExtraArchiveLabel)
		addLabels = append(addLabels, extraLabelId)
	}

	modReq := gm.BatchModifyMessagesRequest{
		AddLabelIds:    addLabels,
		RemoveLabelIds: []string{inboxLabelId},
	}

	return a.helper.BatchModifyMessages(msgs, &modReq)
}

func runArchiveCmd(cmd *cobra.Command, args []string) {
	conf := loadConfig()

	srv := api.NewGmailClient(api.ModifyScope)

	fmt.Print("Fetching inbox... ")
	gHelper := NewGmailHelper(srv, api.DefaultUser)

	arch := NewArchiver(srv, conf, gHelper)
	msgsToArchive := arch.LoadMsgsToArchive()
	fmt.Print("done\n")

	if len(msgsToArchive) > 0 {
		if verbose {
			fmt.Println("Messages to archive:")
			gHelper.PrintMessagesByCategory(msgsToArchive)
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
