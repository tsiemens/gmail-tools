package cmd

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"strings"

	gm "google.golang.org/api/gmail/v1"

	"github.com/spf13/cobra"

	"github.com/tsiemens/gmail-tools/api"
	_ "github.com/tsiemens/gmail-tools/filter"
	"github.com/tsiemens/gmail-tools/prnt"
)

var showTouch = false
var showHeadersOnly = false
var showBrief = false

func decodePartBody(part *gm.MessagePart) string {
	data := part.Body.Data
	decoder := base64.NewDecoder(base64.URLEncoding, strings.NewReader(data))
	buf := new(bytes.Buffer)
	buf.ReadFrom(decoder)
	b := buf.Bytes()
	return string(b[:])
}

func runShowCmd(cmd *cobra.Command, args []string) {
	if showHeadersOnly && showBrief {
		prnt.StderrLog.Fatalln("-b and -H are mutually exclusive")
	}

	msgId := args[0]
	if msgId == "" {
		prnt.StderrLog.Fatalln("Invalid msgId ''")
	}

	conf := LoadConfig()
	srv := api.NewGmailClient(api.ModifyScope)
	gHelper := NewGmailHelper(srv, api.DefaultUser, conf)

	msg, err := gHelper.LoadMessage(msgId)
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
			fmt.Println(decodePartBody(msg.Payload))
			for _, part := range msg.Payload.Parts {
				// For multipart messages
				fmt.Println(decodePartBody(part))
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
