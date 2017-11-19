package cmd

import (
	gm "google.golang.org/api/gmail/v1"

	"github.com/spf13/cobra"
	"github.com/tsiemens/gmail-tools/api"
	"github.com/tsiemens/gmail-tools/prnt"
	"github.com/tsiemens/gmail-tools/util"
)

var searchLabels []string
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
		prnt.StderrLog.Fatalln("-u and -i options are mutually exclusive")
	}

	cache := NewCache()
	cache.LoadMsgs()

	query := ""
	for _, label := range searchLabels {
		query += "label:(" + label + ") "
	}
	if len(args) > 0 {
		query += args[0] + " "
	}

	if query == "" {
		prnt.StderrLog.Println("No query provided")
	}

	conf := LoadConfig()
	if searchTouch && conf.ApplyLabelOnTouch == "" {
		prnt.StderrLog.Fatalf("No ApplyLabelOnTouch property found in %s\n",
			conf.configFile)
	}

	srv := api.NewGmailClient(api.ModifyScope)
	gHelper := NewGmailHelper(srv, api.DefaultUser, conf)

	msgs, err := gHelper.QueryMessages(query, false, false, IdsOnly)
	if err != nil {
		prnt.StderrLog.Fatalf("%v\n", err)
	}
	prnt.LPrintf(prnt.Debug, "Debug: Query returned %d mesages\n", len(msgs))

	hasLoadedMsgDetails := false

	if searchInteresting || searchUninteresting {
		var filteredMsgs []*gm.Message
		msgs, err = gHelper.LoadDetailedUncachedMessages(msgs, cache)
		util.CheckErr(err)
		for _, msg := range msgs {
			msgInterest := gHelper.MsgInterest(msg)
			if (searchInteresting && msgInterest == Interesting) ||
				(searchUninteresting && msgInterest == Uninteresting) {
				filteredMsgs = append(filteredMsgs, msg)
			}
		}
		cache.UpdateMsgs(msgs)
		hasLoadedMsgDetails = true
		msgs = filteredMsgs
	}

	if len(msgs) == 0 {
		prnt.HPrintln(prnt.Always, "Query matched no messages")
		return
	}
	prnt.HPrintf(prnt.Always, "Query matched %d messages\n", len(msgs))

	if !Quiet && MaybeConfirmFromInput("Show messages?", true) {
		if searchPrintIdsOnly {
			for _, msg := range msgs {
				prnt.Printf("%s,%s\n", msg.Id, msg.ThreadId)
			}
		} else {
			if !hasLoadedMsgDetails {
				msgs, err = gHelper.LoadDetailedUncachedMessages(msgs, cache)
				util.CheckErr(err)
				cache.UpdateMsgs(msgs)
				hasLoadedMsgDetails = true
			}

			gHelper.PrintMessagesByCategory(msgs)
		}
	}

	if searchTouch {
		maybeTouchMessages(msgs, gHelper)
	}

	cache.WriteMsgs()
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
	searchCmd.Flags().BoolVarP(&searchTouch, "touch", "t", false,
		"Apply 'touched' label from ~/.gmailcli/config.yaml")
	searchCmd.Flags().BoolVarP(&searchInteresting, "interesting", "i", false,
		"Filter results by interesting messages")
	searchCmd.Flags().BoolVarP(&searchUninteresting, "uninteresting", "u", false,
		"Filter results by uninteresting messages")
	searchCmd.Flags().BoolVar(&searchPrintIdsOnly, "ids-only", false,
		"Only prints out only messageId,threadId (does not prompt)")
	addDryFlag(searchCmd)
}
