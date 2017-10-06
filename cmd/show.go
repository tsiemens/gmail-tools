package cmd

import (
	"fmt"
	"log"
	// "os"
	"bytes"
	"encoding/base64"
	"strings"

	// gm "google.golang.org/api/gmail/v1"

	"github.com/spf13/cobra"
	"github.com/tsiemens/gmail-tools/api"
	// "github.com/tsiemens/gmail-tools/prnt"
)

func runShowCmd(cmd *cobra.Command, args []string) {
	msgId := args[0]

	conf := LoadConfig()
	srv := api.NewGmailClient(api.ReadScope)
	gHelper := NewGmailHelper(srv, api.DefaultUser, conf)

	msg, err := gHelper.LoadMessage(msgId)
	if err != nil {
		log.Fatalf("%v\n", err)
	}

	// gHelper.PrintMessage(msg)
	for _, hdr := range msg.Payload.Headers {
		fmt.Printf("%s: %s\n", hdr.Name, hdr.Value)
	}

	str := msg.Payload.Body.Data
	decoder := base64.NewDecoder(base64.URLEncoding, strings.NewReader(str))
	buf := new(bytes.Buffer)
	buf.ReadFrom(decoder)
	b := buf.Bytes()
	data := string(b[:])

	fmt.Println(data)
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
}
