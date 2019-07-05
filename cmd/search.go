package cmd

import (
	"sort"

	gm "google.golang.org/api/gmail/v1"

	"github.com/spf13/cobra"
	"github.com/tsiemens/gmail-tools/api"
	"github.com/tsiemens/gmail-tools/config"
	"github.com/tsiemens/gmail-tools/prnt"
	"github.com/tsiemens/gmail-tools/util"
)

var searchLabels []string
var searchLabelsToAdd []string
var searchTouch = false
var searchTrash = false
var searchInteresting = false
var searchUninteresting = false
var searchPrintIdsOnly = false
var searchPrintJson = false
var searchMaxMsgs int64
var searchShowSummary = false

func showSummary(msgs []*gm.Message, gHelper *GmailHelper) {
	prnt.Hum.Always.Ln("\nMESSAGE SUMMARY\n")

	type stringCountTup struct {
		Name  string
		Count int
	}

	sortedTupList := func(countMap map[string]int) []stringCountTup {
		stringsSorted := make([]stringCountTup, 0, len(countMap))
		for name, count := range countMap {
			stringsSorted = append(stringsSorted, stringCountTup{name, count})
		}
		sort.Slice(
			stringsSorted,
			func(i, j int) bool { return stringsSorted[i].Count > stringsSorted[j].Count })

		return stringsSorted
	}

	// Show message count per label
	labelCounts := make(map[string]int)
	for _, msg := range msgs {
		labels := gHelper.Msgs.MessageLabelNames(msg)
		for _, label := range labels {
			var count int
			var ok bool
			if count, ok = labelCounts[label]; !ok {
				count = 0
			}
			count++
			labelCounts[label] = count
		}
	}

	labelsSorted := sortedTupList(labelCounts)

	prnt.Hum.Always.Ln("Messages per label:\n-------------------------------")

	for _, tup := range labelsSorted {
		prnt.Hum.Always.F("%-20s %d\n", tup.Name, tup.Count)
	}

	// Show statistics from the message headers like sender and recipient counts
	senderCounts := make(map[string]int)
	recipientCounts := make(map[string]int)
	for _, msg := range msgs {
		headers, err := api.GetMsgHeaders(msg)
		if err != nil {
			prnt.Hum.Always.Ln("Error retreiving message header:", err)
			continue
		}

		var count int
		var ok bool
		if count, ok = senderCounts[headers.From.Address]; !ok {
			count = 0
		}
		count++
		senderCounts[headers.From.Address] = count

		for _, email := range headers.To {
			if count, ok = recipientCounts[email.Address]; !ok {
				count = 0
			}
			count++
			recipientCounts[email.Address] = count
		}
		for _, email := range headers.Cc {
			if count, ok = recipientCounts[email.Address]; !ok {
				count = 0
			}
			count++
			recipientCounts[email.Address] = count
		}
	}

	printCountsWithThresholdOfMax := func(header string, skippedFmt string, countMap map[string]int) {
		sortedTups := sortedTupList(countMap)

		prnt.Hum.Always.Ln(header)

		countThreshold := 0
		if len(sortedTups) > 0 {
			largestFewCnt := util.IntMin(len(sortedTups), 3)
			largestFewTotal := 0
			for i := 0; i < largestFewCnt; i++ {
				largestFewTotal += sortedTups[i].Count
			}
			largestFewAvg := largestFewTotal / largestFewCnt

			// Arbitrary. Chose to limit to 20% of biggest few
			countThreshold = int(float32(largestFewAvg) * 0.20)
		}
		minEntrys := 10
		skippedCnt := 0
		for i, tup := range sortedTups {
			if i < minEntrys ||
				tup.Count >= countThreshold {
				prnt.Hum.Always.F("%-40s %d\n", tup.Name, tup.Count)
			} else {
				skippedCnt++
			}
		}
		if skippedCnt > 0 {
			prnt.Hum.Always.F(skippedFmt, skippedCnt)
		}
	}

	printCountsWithThresholdOfMax(
		"\nMessages sent by:\n-------------------------------",
		"(%d additional senders skipped; send counts too low)\n",
		senderCounts,
	)

	printCountsWithThresholdOfMax(
		"\nMessages sent to:\n-------------------------------",
		"(%d additional recipients skipped; counts too low)\n",
		recipientCounts,
	)
}

func runSearchCmd(cmd *cobra.Command, args []string) {
	if searchInteresting && searchUninteresting {
		prnt.StderrLog.Fatalln("-u and -i options are mutually exclusive")
	}

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

	conf := config.AppConfig()
	if searchTouch && conf.ApplyLabelOnTouch == "" {
		prnt.StderrLog.Fatalf("No ApplyLabelOnTouch property found in %s\n",
			conf.ConfigFile)
	}

	srv := api.NewGmailClient(api.ModifyScope)
	gHelper := NewGmailHelper(srv, api.DefaultUser, conf)

	msgs, err := gHelper.Msgs.QueryMessages(query, false, false, searchMaxMsgs, api.IdsOnly)
	if err != nil {
		prnt.StderrLog.Fatalf("%v\n", err)
	}
	prnt.LPrintf(prnt.Debug, "Debug: Query returned %d mesages\n", len(msgs))

	hasLoadedMsgDetails := false

	requiredDetail := gHelper.RequiredDetailForPluginInterest()

	if searchInteresting || searchUninteresting {
		var interest InterestCategory
		if searchInteresting {
			interest = Interesting
		} else {
			interest = Uninteresting
		}
		msgs, err = gHelper.FilterMessagesByInterest(interest, msgs)
		util.CheckErr(err)

		hasLoadedMsgDetails = true
	}

	if len(msgs) == 0 {
		prnt.HPrintln(prnt.Always, "Query matched no messages")
		return
	}
	prnt.HPrintf(prnt.Always, "Query matched %d messages\n", len(msgs))

	if searchShowSummary {
		if !hasLoadedMsgDetails {
			msgs, err = gHelper.Msgs.LoadMessages(msgs, requiredDetail)
			util.CheckErr(err)
			hasLoadedMsgDetails = true
		}

		showSummary(msgs, gHelper)
	} else if !Quiet && MaybeConfirmFromInput("Show messages?", true) {
		if searchPrintIdsOnly {
			for _, msg := range msgs {
				prnt.Printf("%s,%s\n", msg.Id, msg.ThreadId)
			}
		} else {
			if !hasLoadedMsgDetails {
				msgs, err = gHelper.Msgs.LoadMessages(msgs, requiredDetail)
				util.CheckErr(err)
				hasLoadedMsgDetails = true
			}

			if searchPrintJson {
				gHelper.PrintMessagesJson(msgs)
			} else {
				gHelper.PrintMessagesByCategory(msgs)
			}
		}
	}

	if len(searchLabelsToAdd) > 0 {
		maybeApplyLabels(msgs, gHelper, searchLabelsToAdd)
	}
	if searchTouch {
		maybeTouchMessages(msgs, gHelper)
	}
	if searchTrash {
		maybeTrashMessages(msgs, gHelper)
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
	searchCmd.Flags().StringArrayVar(&searchLabelsToAdd, "add-label", []string{},
		"Apply a label to matches (may be provided multiple times)")
	searchCmd.Flags().BoolVarP(&searchTouch, "touch", "t", false,
		"Apply 'touched' label from ~/.gmailcli/config.yaml")
	searchCmd.Flags().BoolVar(&searchTrash, "trash", false,
		"Send messages to the trash")
	searchCmd.Flags().BoolVarP(&searchInteresting, "interesting", "i", false,
		"Filter results by interesting messages")
	searchCmd.Flags().BoolVarP(&searchUninteresting, "uninteresting", "u", false,
		"Filter results by uninteresting messages")
	searchCmd.Flags().BoolVar(&searchPrintIdsOnly, "ids-only", false,
		"Only prints out only messageId,threadId (does not prompt)")
	searchCmd.Flags().BoolVar(&searchPrintJson, "json", false,
		"Print message details formatted as json")
	searchCmd.Flags().BoolVar(&searchShowSummary, "summary", false,
		"Print a statistical summary of the matched messages")
	searchCmd.Flags().Int64VarP(&searchMaxMsgs, "max", "m", -1,
		"Set a max on how many results are queried.")
	addDryFlag(searchCmd)
}
