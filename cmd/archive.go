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
)

const (
	caseIgnore = "(?i)"
)

func messageLabelNames(m *gm.Message, labels map[string]string) []string {
	var lNames []string
	for _, lId := range m.LabelIds {
		lNames = append(lNames, labels[lId])
	}
	return lNames
}

func printMessage(m *gm.Message, labels map[string]string) {
	fmt.Printf("[%s]\n", strings.Join(messageLabelNames(m, labels), ", "))
	if m.Payload == nil || m.Payload.Headers == nil {
		fmt.Println("<No subject>")
		return
	}
	for _, hdr := range m.Payload.Headers {
		if hdr.Name == "Subject" {
			fmt.Printf("%s\n", hdr.Value)
			break
		}
	}
}

type ConfigModel struct {
	DoNotIgnoreQuery         string   `yaml:"DoNotIgnoreQuery"`
	IgnoreLabelPatterns      []string `yaml:"IgnoreLabelPatterns"`
	DoNotIgnoreLabelPatterns []string `yaml:"DoNotIgnoreLabelPatterns"`

	ignoreLabelRegexps []*regexp.Regexp
	dniLabelRegexps    []*regexp.Regexp
}

func loadConfig() *ConfigModel {
	confData, err := ioutil.ReadFile("./config.yaml")
	if err != nil {
		log.Fatalf("Could not open. %v", err)
	}

	conf := &ConfigModel{}
	err = yaml.Unmarshal(confData, conf)
	if err != nil {
		log.Fatalf("Could not unmarshal: %v", err)
	}
	fmt.Printf("config: %+v\n", conf)

	for _, pat := range conf.IgnoreLabelPatterns {
		re, err := regexp.Compile(caseIgnore + pat)
		if err != nil {
			break
		}
		conf.ignoreLabelRegexps = append(conf.ignoreLabelRegexps, re)
	}
	if err == nil {
		for _, pat := range conf.DoNotIgnoreLabelPatterns {
			re, err := regexp.Compile(caseIgnore + pat)
			if err != nil {
				break
			}
			conf.dniLabelRegexps = append(conf.dniLabelRegexps, re)
		}
	}
	if err != nil {
		log.Fatalf("Failed to load config: \"%s\"", err)
	}
	return conf
}

func getLabels(srv *gm.Service) map[string]string {
	user := "me"
	r, err := srv.Users.Labels.List(user).Do()
	if err != nil {
		log.Fatalf("Unable to retrieve labels. %v", err)
	}
	labelMap := make(map[string]string)
	for _, l := range r.Labels {
		labelMap[l.Id] = l.Name
	}
	return labelMap
}

func shouldArchive(m *gm.Message, labels map[string]string, conf *ConfigModel) bool {
	var matchedIgnored = false
	var matchedDni = false

	for _, lId := range m.LabelIds {
		lName := labels[lId]
		labelIgnored := false
		for _, labRe := range conf.ignoreLabelRegexps {
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
		for _, labRe := range conf.dniLabelRegexps {
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

func runArchiveCmd(cmd *cobra.Command, args []string) {
	conf := loadConfig()

	srv := api.NewGmailClient(api.ReadScope)

	labels := getLabels(srv)

	user := "me"
	r, err := srv.Users.Messages.List(user).
		Q("in:inbox -(" + conf.DoNotIgnoreQuery + ")").Do()
	if err != nil {
		log.Fatalf("Unable to retrieve threads. %v", err)
	}

	var msgsToArchive []*gm.Message
	for _, m := range r.Messages {
		msg, err := srv.Users.Messages.Get(user, m.Id).Do()
		if err != nil {
			log.Fatal("Failed to get message")
		}
		if shouldArchive(msg, labels, conf) {
			msgsToArchive = append(msgsToArchive, msg)
		}
	}

	if len(msgsToArchive) > 0 {
		fmt.Println("Messages to archive")
		for _, m := range msgsToArchive {
			printMessage(m, labels)
			fmt.Println("")
		}
	} else {
		fmt.Println("No messages to archive")
	}
}

// archiveCmd represents the archive command
var archiveCmd = &cobra.Command{
	Use:   "archive",
	Short: "Attempts to archive unread threads in the inbox, based on the rules in TODO",
	Run:   runArchiveCmd,
}

func init() {
	RootCmd.AddCommand(archiveCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// archiveCmd.PersistentFlags().String("foo", "", "A help for foo")

	archiveCmd.Flags().BoolP("dry", "n", false,
		"Perform no action, just print what would be done")
}
