package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	gm "google.golang.org/api/gmail/v1"

	"github.com/tsiemens/gmail-tools/api"
	"github.com/tsiemens/gmail-tools/config"
	"github.com/tsiemens/gmail-tools/prnt"
	"github.com/tsiemens/gmail-tools/util"
)

const (
	inboxLabelId = "INBOX"
)

var archiveRead = false
var archiveOutdated = false

type Archiver struct {
	srv    *gm.Service
	conf   *config.Config
	helper *GmailHelper
}

func NewArchiver(srv *gm.Service, conf *config.Config, helper *GmailHelper) *Archiver {
	if srv == nil || conf == nil || helper == nil {
		prnt.StderrLog.Fatalln("Internal error creating Archiver: ", srv, conf, helper)
	}
	return &Archiver{srv: srv, conf: conf, helper: helper}
}

func (a *Archiver) LoadMsgsToArchive(query string) []*gm.Message {
	if query == "" {
		query = " -(" + a.conf.InterestingMessageQuery + ")"
	}

	detail := a.helper.RequiredDetailForPluginInterest()
	var maxMsgs int64 = -1
	msgs, err := a.helper.Msgs.QueryMessages(query,
		true, !archiveRead, maxMsgs, detail)
	util.CheckErr(err)

	msgsToArchive, err := a.helper.FilterMessagesByInterest(Uninteresting, msgs)
	util.CheckErr(err)
	return msgsToArchive
}

func (a *Archiver) LoadOutdatedMsgsToArchive(query string) []*gm.Message {
	if query == "" {
		query = "-(category:primary)"
	}

	if !archiveRead {
		query += " label:unread"
	}

	return a.helper.FindOutdatedMessages(query)
}

func (a *Archiver) ArchiveMessages(msgs []*gm.Message) error {
	var addLabels []string

	var extraLabelId string
	if a.conf.ApplyLabelToUninteresting != "" {
		extraLabelId = a.helper.Msgs.LabelIdFromName(a.conf.ApplyLabelToUninteresting)
		addLabels = append(addLabels, extraLabelId)
	}

	modReq := gm.BatchModifyMessagesRequest{
		AddLabelIds:    addLabels,
		RemoveLabelIds: []string{inboxLabelId},
	}

	return a.helper.Msgs.BatchModifyMessages(msgs, &modReq)
}

func runArchiveCmd(cmd *cobra.Command, args []string) {
	query := ""
	if len(args) > 0 {
		query += args[0] + " "
	}

	conf := config.AppConfig()

	srv := api.NewGmailClient(api.ModifyScope)

	prnt.HPrint(prnt.Always, "Fetching inbox... ")
	gHelper := NewGmailHelper(srv, api.DefaultUser, conf)

	arch := NewArchiver(srv, conf, gHelper)
	var msgsToArchive []*gm.Message
	if archiveOutdated {
		msgsToArchive = arch.LoadOutdatedMsgsToArchive(query)
	} else {
		msgsToArchive = arch.LoadMsgsToArchive(query)
	}
	prnt.HPrint(prnt.Always, "done\n")

	if len(msgsToArchive) > 0 {
		if Verbose {
			fmt.Println("Messages to archive:")
			gHelper.PrintMessagesByCategory(msgsToArchive)
			fmt.Print("\n")
		}

		fmt.Printf("Message count: %d\n", len(msgsToArchive))
		if conf.ApplyLabelToUninteresting != "" {
			fmt.Printf("Extra label %s will be applied.\n", conf.ApplyLabelToUninteresting)
		}

		if DryRun {
			prnt.LPrintln(prnt.Quietable, "Skipping committing changes (--dry provided)")
		} else {
			if util.ConfirmFromInput("Archive these?", false) {
				err := arch.ArchiveMessages(msgsToArchive)
				if err != nil {
					prnt.StderrLog.Fatalf("Failed to archive messages: %s\n", err)
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
	Use: "archive [BASE_QUERY]",
	Short: "Attempts to archive unread messages in the inbox, based on the rules " +
		"in ~/.gmailcli/config.yaml. Optionally, the base query can be provided " +
		"to override the default.",
	Aliases: []string{"arch"},
	Run:     runArchiveCmd,
	Args:    cobra.RangeArgs(0, 1),
}

func init() {
	RootCmd.AddCommand(archiveCmd)

	addDryFlag(archiveCmd)
	archiveCmd.Flags().BoolVarP(&archiveRead, "include-read", "r", false,
		"Archive read and unread inbox messages")
	archiveCmd.Flags().BoolVarP(&archiveOutdated, "outdated", "o", false,
		"Find and archive outdated messages only (duplicates, obsolete, etc.)")
}
