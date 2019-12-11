package cmd

import (
	"strings"

	gm "google.golang.org/api/gmail/v1"

	"errors"
	"github.com/spf13/cobra"
	"github.com/tsiemens/gmail-tools/api"
	"github.com/tsiemens/gmail-tools/config"
	"github.com/tsiemens/gmail-tools/plugin"
	"github.com/tsiemens/gmail-tools/prnt"
	"github.com/tsiemens/gmail-tools/searchutil"
	"github.com/tsiemens/gmail-tools/util"
)

var searchLabels []string
var searchLabelsToAdd []string
var searchTouch = false
var searchTrash = false
var searchInteresting = false
var searchUninteresting = false
var searchDumpCustomFilters = false
var searchCustomFilterNames []string
var searchInverseCustomFilterNames []string
var searchPrintIdsOnly = false
var searchPrintJson = false
var searchMaxMsgs int64
var searchShowSummary = false

func showSummary(msgs []*gm.Message, gHelper *GmailHelper) {
	prnt.Hum.Always.Ln("\nMESSAGE SUMMARY\n")

	// Show message count per label
	labelCounts := searchutil.NewCountedStringDefaultMap()
	for _, msg := range msgs {
		labels := gHelper.Msgs.MessageLabelNames(msg)
		for _, label := range labels {
			labelCounts.Inc(label)
		}
	}

	labelsSorted := searchutil.MapToSortedCountedStrings(labelCounts.Map)

	// prnt.Hum.Always.Ln("Messages per label:\n-------------------------------")
	prnt.Hum.Always.F("Messages per label:\n%s\n", strings.Repeat("-", 30))

	for _, tup := range labelsSorted {
		prnt.Hum.Always.F("%-20s %d\n", tup.Str, tup.Count)
	}

	// Show statistics from the message headers like sender and recipient counts
	senderCounts := searchutil.NewCountedStringDefaultMap()
	recipientCounts := searchutil.NewCountedStringDefaultMap()
	for _, msg := range msgs {
		headers, err := api.GetMsgHeaders(msg)
		if err != nil {
			prnt.Hum.Always.Ln("Error retreiving message header:", err)
			continue
		}

		senderCounts.Inc(headers.From.Address)

		for _, email := range headers.To {
			recipientCounts.Inc(email.Address)
		}
		for _, email := range headers.Cc {
			recipientCounts.Inc(email.Address)
		}
	}

	searchutil.PrintCountsWithThresholdOfMax(
		"\nMessages sent by:\n-------------------------------",
		"senders",
		10, // Show at least 10
		20, // Show those with at least 20% of counts of top 3
		senderCounts.Map,
	)

	searchutil.PrintCountsWithThresholdOfMax(
		"\nMessages sent to:\n-------------------------------",
		"recipients",
		10, // Show at least 10
		20, // Show those with at least 20% of counts of top 3
		recipientCounts.Map,
	)

	// Show plugin-specific summaries
	plugins := gHelper.GetPlugins()
	for _, plug := range plugins {
		if plug.PrintMessageSummary != nil {
			plug.PrintMessageSummary(msgs, gHelper.Msgs)
		}
	}
}

func dumpExtraFilters(gHelper *GmailHelper) {
	plugins := gHelper.GetPlugins()
	for _, plug := range plugins {
		if plug.MessageFilters != nil {
			prnt.Hum.Always.F("From %s:\n", plug.Name)
			for name := range plug.MessageFilters {
				filter := plug.MessageFilters[name]
				prnt.Hum.Always.F("%-30s %s\n", name, filter.Desc)
			}
		}
	}
}

func applyCustomFilters(msgs []*gm.Message, gHelper *GmailHelper) []*gm.Message {
	allFilters := make(map[string]*plugin.MessageFilter)
	plugins := gHelper.GetPlugins()
	for _, plug := range plugins {
		if plug.MessageFilters != nil {
			for name := range plug.MessageFilters {
				allFilters[name] = plug.MessageFilters[name]
			}
		}
	}

	type filterAndDirection struct {
		Filter *plugin.MessageFilter
		Invert bool
	}

	filtersToApply := make([]filterAndDirection, 0, len(searchCustomFilterNames))
	for _, name := range searchCustomFilterNames {
		if filter, ok := allFilters[name]; ok {
			prnt.Deb.Ln("Will apply filter", name)
			filtersToApply = append(filtersToApply, filterAndDirection{filter, false})
		} else {
			prnt.StderrLog.Fatalf(
				"'%s' is not an available xfilter. Run search --list-xfilters for available filters.\n", name)
		}
	}

	for _, name := range searchInverseCustomFilterNames {
		if filter, ok := allFilters[name]; ok {
			prnt.Deb.Ln("Will inversely apply filter", name)
			filtersToApply = append(filtersToApply, filterAndDirection{filter, true})
		} else {
			prnt.StderrLog.Fatalf(
				"'%s' is not an available xfilter. Run search --list-xfilters for available filters.\n", name)
		}
	}

	prnt.Hum.Always.P("Running extra filters on messages ")
	filteredMsgs := make([]*gm.Message, 0)

	querySem := make(chan bool, api.MaxConcurrentRequests)
	includeMsgChan := make(chan *gm.Message, 100)
	excludeMsgChan := make(chan *gm.Message, 100)

	for _, msg_ := range msgs {
		go func(msg *gm.Message) {
			querySem <- true
			defer func() { <-querySem }()
			for _, fAndD := range filtersToApply {
				if fAndD.Filter.Matches(msg, gHelper.Msgs) == fAndD.Invert {
					excludeMsgChan <- msg
					return
				}
			}
			// Matched all filters
			includeMsgChan <- msg
		}(msg_)
	}

	progP := prnt.NewProgressPrinter(len(msgs))
	for i := 0; i < len(msgs); i++ {
		progP.Progress(1)
		select {
		case msg := <-includeMsgChan:
			filteredMsgs = append(filteredMsgs, msg)
		case <-excludeMsgChan:
		}
	}

	prnt.Hum.Always.P("\n")
	return filteredMsgs
}

func runSearchCmd(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true
	if searchInteresting && searchUninteresting {
		prnt.StderrLog.Fatalln("-u and -i options are mutually exclusive")
	}

	conf := config.AppConfig()
	if searchTouch && conf.ApplyLabelOnTouch == "" {
		prnt.StderrLog.Fatalf("No ApplyLabelOnTouch property found in %s\n",
			conf.ConfigFile)
	}

	srv := api.NewGmailClient(api.ModifyScope)
	gHelper := NewGmailHelper(srv, api.DefaultUser, conf)

	// Special options, which don't search
	if searchDumpCustomFilters {
		dumpExtraFilters(gHelper)
		return nil
	}

	// Proceed with normal command
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

	if len(searchCustomFilterNames) > 0 || len(searchInverseCustomFilterNames) > 0 {
		msgs = applyCustomFilters(msgs, gHelper)
	}

	if len(msgs) == 0 {
		return errors.New("Query matched no messages")
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
	return nil
}

var searchCmd = &cobra.Command{
	Use:     "search [QUERY]",
	Short:   "Searches for messages with the given query",
	Aliases: []string{"find"},
	RunE:    runSearchCmd,
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
	searchCmd.Flags().BoolVar(&searchDumpCustomFilters, "list-xfilters", false,
		"List the names of all available extra filters")
	searchCmd.Flags().StringArrayVarP(&searchCustomFilterNames, "xfilter", "f",
		[]string{},
		"Extra filters to apply. May be loaded from plugins. "+
			"(may be provided multiple times)")
	searchCmd.Flags().StringArrayVarP(&searchInverseCustomFilterNames, "not-xfilter", "F",
		[]string{},
		"Extra filters to apply inversely. May be loaded from plugins. "+
			"(may be provided multiple times)")
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
