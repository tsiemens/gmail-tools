package cmd

import (
	"regexp"

	"github.com/spf13/cobra"

	"github.com/tsiemens/gmail-tools/api"
	"github.com/tsiemens/gmail-tools/prnt"
)

func runDummyCmd(cmd *cobra.Command, args []string) {
	prnt.Println("Dummy command")
}

func runListFilterCmd(cmd *cobra.Command, args []string) {
	var searchRegexp *regexp.Regexp
	if len(args) > 0 {
		var err error
		searchRegexp, err = regexp.Compile(args[0])
		if err != nil {
			prnt.StderrLog.Fatalln("Failed to compile regex pattern:", err)
		}
	}

	conf := LoadConfig()
	srv := api.NewGmailClient(api.ModifyScope)
	gHelper := NewGmailHelper(srv, api.DefaultUser, conf)
	filters, err := gHelper.GetFilters()
	if err != nil {
		prnt.StderrLog.Fatalln("Error getting filters:", err)
	}
	if len(filters) == 0 {
		prnt.LPrintln(prnt.Quietable, "No filters found")
	}
	for _, filter := range filters {
		if searchRegexp == nil || gHelper.MatchesFilter(searchRegexp, filter) {
			gHelper.PrintFilter(filter)
		}
	}
}

// filterCmd represents the filter command tree
var filterCmd = &cobra.Command{
	Use:     "filter",
	Short:   "Filter related commands",
	Aliases: []string{"fi", "fl"},
}

var listFilterCmd = &cobra.Command{
	Use:   "list [SEARCH REGEXP]",
	Short: "List Gmail filters",
	Long: "List Gmail filters\n\nArgs:\n" +
		"SEARCH REGEXP - Only show filters matching this pattern",
	Aliases: []string{"ls"},
	Args:    cobra.MaximumNArgs(1),
	Run:     runListFilterCmd,
}

func init() {
	filterCmd.AddCommand(listFilterCmd)
	RootCmd.AddCommand(filterCmd)

	// addDryFlag(listFilterCmd)

	// filter list
}
