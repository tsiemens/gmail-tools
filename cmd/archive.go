package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	gm "google.golang.org/api/gmail/v1"

	"github.com/tsiemens/gmail-tools/api"
	"github.com/tsiemens/gmail-tools/prnt"
	"github.com/tsiemens/gmail-tools/util"
)

const (
	inboxLabelId = "INBOX"
)

var archiveRead = false

type Archiver struct {
	srv    *gm.Service
	conf   *Config
	helper *GmailHelper
}

func NewArchiver(srv *gm.Service, conf *Config, helper *GmailHelper) *Archiver {
	if srv == nil || conf == nil || helper == nil {
		prnt.StderrLog.Fatalln("Internal error creating Archiver: ", srv, conf, helper)
	}
	return &Archiver{srv: srv, conf: conf, helper: helper}
}

func (a *Archiver) LoadMsgsToArchive() []*gm.Message {
	var maxMsgs int64 = -1
	msgs, err := a.helper.QueryMessages(" -("+a.conf.InterestingMessageQuery+")",
		true, !archiveRead, maxMsgs, LabelsAndPayload)
	util.CheckErr(err)

	var msgsToArchive []*gm.Message
	for _, msg := range msgs {
		if a.helper.MsgInterest(msg) == Uninteresting {
			msgsToArchive = append(msgsToArchive, msg)
		}
	}
	return msgsToArchive
}

func (a *Archiver) ArchiveMessages(msgs []*gm.Message) error {
	var addLabels []string

	var extraLabelId string
	if a.conf.ApplyLabelToUninteresting != "" {
		extraLabelId = a.helper.LabelIdFromName(a.conf.ApplyLabelToUninteresting)
		addLabels = append(addLabels, extraLabelId)
	}

	modReq := gm.BatchModifyMessagesRequest{
		AddLabelIds:    addLabels,
		RemoveLabelIds: []string{inboxLabelId},
	}

	return a.helper.BatchModifyMessages(msgs, &modReq)
}

func runArchiveCmd(cmd *cobra.Command, args []string) {
	conf := LoadConfig()

	srv := api.NewGmailClient(api.ModifyScope)

	prnt.HPrint(prnt.Always, "Fetching inbox... ")
	gHelper := NewGmailHelper(srv, api.DefaultUser, conf)

	arch := NewArchiver(srv, conf, gHelper)
	msgsToArchive := arch.LoadMsgsToArchive()
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
	Use: "archive",
	Short: "Attempts to archive unread messages in the inbox, based on the rules " +
		"in ~/.gmailcli/config.yaml",
	Aliases: []string{"arch"},
	Run:     runArchiveCmd,
}

func init() {
	RootCmd.AddCommand(archiveCmd)

	addDryFlag(archiveCmd)
	archiveCmd.Flags().BoolVarP(&archiveRead, "include-read", "r", false,
		"Archive read and unread inbox messages")
}
