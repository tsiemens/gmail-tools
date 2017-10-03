package cmd

import (
	"fmt"
	"log"

	"github.com/spf13/cobra"
	"github.com/tsiemens/gmail-tools/api"
	"github.com/tsiemens/gmail-tools/util"
)

var searchLabels []string

func runSearchCmd(cmd *cobra.Command, args []string) {
	srv := api.NewGmailClient(api.ReadScope)
	gHelper := NewGmailHelper(srv, api.DefaultUser, nil)

	query := ""
	for _, label := range searchLabels {
		query += "label:(" + label + ") "
	}
	if len(args) > 0 {
		query += args[0] + " "
	}
	msgs, err := gHelper.QueryMessages(query, false, false, IdsOnly)
	if err != nil {
		log.Fatalf("%v\n", err)
	}

	if len(msgs) == 0 {
		fmt.Println("Query matched no messages")
		return
	}
	fmt.Printf("Query matched %d messages\n", len(msgs))

	if util.ConfirmFromInput("Show messages?", true) {
		msgs, err = gHelper.LoadDetailedMessages(msgs, LabelsAndPayload)
		if err != nil {
			log.Fatalf("%v\n", err)
		}
		gHelper.PrintMessagesByCategory(msgs)
	}
}

var searchCmd = &cobra.Command{
	Use:     "search [QUERY]",
	Short:   "Searches for messages with the given query",
	Aliases: []string{"find"},
	Run:     runSearchCmd,
	Args:    cobra.RangeArgs(0, 1),
}

var searchQuiet = false

func init() {
	RootCmd.AddCommand(searchCmd)

	searchCmd.Flags().StringArrayVarP(&searchLabels, "labelp", "l", []string{},
		"Label regexps to match in the search (may be provided multiple times)")
	searchCmd.Flags().BoolVarP(&searchQuiet, "quiet", "q", false,
		"Quiet")
	addDryFlag(searchCmd)
}
