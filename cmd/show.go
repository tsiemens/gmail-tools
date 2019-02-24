package cmd

import (
	"fmt"

	gm "google.golang.org/api/gmail/v1"

	"github.com/spf13/cobra"

	"github.com/tsiemens/gmail-tools/api"
	"github.com/tsiemens/gmail-tools/config"
	_ "github.com/tsiemens/gmail-tools/filter"
	"github.com/tsiemens/gmail-tools/prnt"
)

var showTouch = false
var showHeadersOnly = false
var showBrief = false

func runShowCmd(cmd *cobra.Command, args []string) {
	if showHeadersOnly && showBrief {
		prnt.StderrLog.Fatalln("-b and -H are mutually exclusive")
	}

	msgId := args[0]
	if msgId == "" {
		prnt.StderrLog.Fatalln("Invalid msgId ''")
	}

	conf := config.AppConfig()
	srv := api.NewGmailClient(api.ModifyScope)
	gHelper := NewGmailHelper(srv, api.DefaultUser, conf)

	msg, err := gHelper.Msgs.GetMessage(msgId, api.LabelsAndPayload)
	if err != nil {
		prnt.StderrLog.Fatalf("%v\n", err)
	}

	if showBrief {
		gHelper.PrintMessage(msg)
	} else {
		for _, hdr := range msg.Payload.Headers {
			fmt.Printf("%s: %s\n", hdr.Name, hdr.Value)
		}

		if !showHeadersOnly {
			for _, part := range api.GetMessageBody(msg) {
				fmt.Println(part)
			}
		}
	}

	if showTouch {
		maybeTouchMessages([]*gm.Message{msg}, gHelper)
	}
}

var showCmd = &cobra.Command{
	Use:     "show [MESSAGE_ID]",
	Short:   "Shows details for the message ID",
	Aliases: []string{"sh"},
	Run:     runShowCmd,
	Args:    cobra.ExactArgs(1),
}

func init() {
	RootCmd.AddCommand(showCmd)

	showCmd.Flags().BoolVarP(&showTouch, "touch", "t", false,
		"Mark message as touched")
	showCmd.Flags().BoolVarP(&showHeadersOnly, "headers-only", "H", false,
		"Don't print the message body")
	showCmd.Flags().BoolVarP(&showBrief, "brief", "b", false,
		"Print only a brief summary of the message")
	addDryFlag(showCmd)
	addAssumeYesFlag(showCmd)
}
