package cmd

import (
	"fmt"
	"log"
	"os"

	gm "google.golang.org/api/gmail/v1"

	"github.com/spf13/cobra"
	"github.com/tsiemens/gmail-tools/api"
	"github.com/tsiemens/gmail-tools/util"
)

var searchLabels []string
var searchQuiet = false
var searchTouch = false
var searchInteresting = false
var searchUninteresting = false
var searchPrintIdsOnly = false

func touchMessages(msgs []*gm.Message, gHelper *GmailHelper, conf *Config) error {
	touchLabelId := gHelper.LabelIdFromName(conf.ApplyLabelOnTouch)

	modReq := gm.BatchModifyMessagesRequest{
		AddLabelIds: []string{touchLabelId},
	}
	return gHelper.BatchModifyMessages(msgs, &modReq)
}

func runSearchCmd(cmd *cobra.Command, args []string) {
	if searchInteresting && searchUninteresting {
		fmt.Println("-u and -i options are mutually exclusive")
		os.Exit(1)
	}

	query := ""
	for _, label := range searchLabels {
		query += "label:(" + label + ") "
	}
	if len(args) > 0 {
		query += args[0] + " "
	}

	if query == "" {
		fmt.Println("No query provided")
		os.Exit(1)
	}

	// If the quiet label is set, then we will never need the payload during the
	// command execution.
	var detailLevel MessageDetailLevel
	if searchQuiet {
		detailLevel = LabelsOnly
	} else {
		detailLevel = LabelsAndPayload
	}

	conf := LoadConfig()
	if searchTouch && conf.ApplyLabelOnTouch == "" {
		fmt.Printf("No ApplyLabelOnTouch property found in %s\n", conf.configFile)
		os.Exit(1)
	}

	srv := api.NewGmailClient(api.ModifyScope)
	gHelper := NewGmailHelper(srv, api.DefaultUser, conf)

	msgs, err := gHelper.QueryMessages(query, false, false, IdsOnly)
	if err != nil {
		log.Fatalf("%v\n", err)
	}
	util.Debugf("Debug: Query returned %d mesages\n", len(msgs))

	hasLoadedMsgDetails := false

	if searchInteresting || searchUninteresting {
		var filteredMsgs []*gm.Message
		msgs, err = gHelper.LoadDetailedMessages(msgs, detailLevel)
		for _, msg := range msgs {
			msgInterest := gHelper.MsgInterest(msg)
			if (searchInteresting && msgInterest == Interesting) ||
				(searchUninteresting && msgInterest == Uninteresting) {
				filteredMsgs = append(filteredMsgs, msg)
			}
		}
		hasLoadedMsgDetails = true
		msgs = filteredMsgs
	}

	if len(msgs) == 0 {
		fmt.Println("Query matched no messages")
		return
	}
	fmt.Printf("Query matched %d messages\n", len(msgs))

	if !searchQuiet && MaybeConfirmFromInput("Show messages?", true) {
		if searchPrintIdsOnly {
			for _, msg := range msgs {
				fmt.Println(msg.Id)
			}
		} else {
			if !hasLoadedMsgDetails {
				msgs, err = gHelper.LoadDetailedMessages(msgs, LabelsAndPayload)
				if err != nil {
					log.Fatalf("%v\n", err)
				}
				hasLoadedMsgDetails = true
			}
			gHelper.PrintMessagesByCategory(msgs)
		}
	}

	if searchTouch {
		if DryRun {
			fmt.Println("Skipping touching messages (--dry provided)")
		} else {
			if MaybeConfirmFromInput("Mark messages touched?", false) {
				err := touchMessages(msgs, gHelper, conf)
				if err != nil {
					log.Fatalf("Failed to touch messages: %s\n", err)
				} else {
					fmt.Println("Messages marked touched")
				}
			}
		}
	}
}

var searchCmd = &cobra.Command{
	Use:     "search [QUERY]",
	Short:   "Searches for messages with the given query",
	Aliases: []string{"find"},
	Run:     runSearchCmd,
	Args:    cobra.RangeArgs(0, 1),
}

func init() {
	RootCmd.AddCommand(searchCmd)

	searchCmd.Flags().StringArrayVarP(&searchLabels, "labelp", "l", []string{},
		"Label regexps to match in the search (may be provided multiple times)")
	searchCmd.Flags().BoolVarP(&searchQuiet, "quiet", "q", false,
		"Don't print searched messages")
	searchCmd.Flags().BoolVarP(&searchTouch, "touch", "t", false,
		"Apply 'touched' label from ~/.gmailcli/config.yaml")
	searchCmd.Flags().BoolVarP(&searchInteresting, "interesting", "i", false,
		"Filter results by interesting messages")
	searchCmd.Flags().BoolVarP(&searchUninteresting, "uninteresting", "u", false,
		"Filter results by uninteresting messages")
	searchCmd.Flags().BoolVar(&searchPrintIdsOnly, "ids-only", false,
		"Only prints out the message IDs (does not prompt)")
	addDryFlag(searchCmd)
}
