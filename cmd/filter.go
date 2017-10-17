package cmd

import (
	"regexp"
	"strings"

	"github.com/spf13/cobra"
	gm "google.golang.org/api/gmail/v1"

	"github.com/tsiemens/gmail-tools/api"
	"github.com/tsiemens/gmail-tools/filter"
	"github.com/tsiemens/gmail-tools/filter/template"
	"github.com/tsiemens/gmail-tools/prnt"
)

func copyFilterAndCriteria(filter *gm.Filter) *gm.Filter {
	// Make a copy of the filter, and important pointers
	updatedFilter := &gm.Filter{}
	*updatedFilter = *filter
	updatedFilter.Criteria = &gm.FilterCriteria{}
	*updatedFilter.Criteria = *filter.Criteria
	return updatedFilter
}

func maybeDoFilterChanges(gHelper *GmailHelper, updatedFilters []*gm.Filter) {
	if DryRun {
		prnt.LPrintln(prnt.Quietable, "Skipping committing changes (--dry provided)")
		return
	}

	if len(updatedFilters) == 0 {
		prnt.LPrintln(prnt.Verbose, "No updates to be made")
	} else {
		commit := false
		confirmStr := "Make these changes?"
		if len(updatedFilters) > 1 {
			commit = MaybeConfirmFromInputLong(confirmStr)
		} else {
			commit = MaybeConfirmFromInput(confirmStr, false)
		}
		if commit {
			for _, fltr := range updatedFilters {
				id := fltr.Id
				createdFltr, err := gHelper.CreateFilter(fltr)
				if err != nil {
					prnt.StderrLog.Fatalln("Failed to create filter:", err)
				}
				prnt.Printf("Created filter %s\n", createdFltr.Id)
				err = gHelper.DeleteFilter(id)
				if err != nil {
					prnt.StderrLog.Fatalf("Failed to delete filter %s: %v\n", id, err)
				}
				prnt.Printf("Deleted filter %s\n", id)
			}
		}
	}
}

var replaceFilterField string

func runReplaceFilterCmd(cmd *cobra.Command, args []string) {
	// Check --field flag
	fieldIsValid := false
	for _, attr := range CriteriaAttrs {
		if attr == replaceFilterField {
			fieldIsValid = true
			break
		}
	}
	if !fieldIsValid {
		prnt.StderrLog.Fatalf("\"%s\" is not a valid criteria field.\n"+
			"Valid fields are: %s",
			replaceFilterField, strings.Join(CriteriaAttrs, ", "))
	}

	regexpStr := args[0]
	regexPat, err := regexp.Compile(regexpStr)
	if err != nil {
		prnt.StderrLog.Fatalln("Regexp error:", err)
	}
	replStr := args[1]

	conf := LoadConfig()
	srv := api.NewGmailClient(api.FiltersScope)
	gHelper := NewGmailHelper(srv, api.DefaultUser, conf)
	filters, err := gHelper.GetFilters()
	if err != nil {
		prnt.StderrLog.Fatalln("Error getting filters:", err)
	}

	// Used for diff
	var matchedFilters []*gm.Filter
	var replacementFilters []*gm.Filter

	for _, filter := range filters {
		attrStr := GetFieldAttrString(filter.Criteria, replaceFilterField)
		if regexPat.MatchString(attrStr) {
			matchedFilters = append(matchedFilters, filter)
			updatedFilter := copyFilterAndCriteria(filter)
			SetFieldAttrFromString(updatedFilter.Criteria, replaceFilterField,
				regexPat.ReplaceAllString(attrStr, replStr))
			replacementFilters = append(replacementFilters, updatedFilter)
		}
	}

	if len(replacementFilters) == 0 {
		prnt.LPrintln(prnt.Verbose, "No updates to be made")
		return
	}

	prnt.LPrintln(prnt.Quietable, prnt.Colorize("Replacements to be done:", "bold"))
	for i, newFilter := range replacementFilters {
		oldFilter := matchedFilters[i]
		gHelper.PrintFilterDiff(oldFilter, newFilter)
		prnt.LPrintln(prnt.Quietable, "")
	}

	maybeDoFilterChanges(gHelper, replacementFilters)
}

func runUpdateFilterCmd(cmd *cobra.Command, args []string) {
	conf := LoadConfig()
	srv := api.NewGmailClient(api.FiltersScope)
	gHelper := NewGmailHelper(srv, api.DefaultUser, conf)
	filters, err := gHelper.GetFilters()
	if err != nil {
		prnt.StderrLog.Fatalln("Error getting filters:", err)
	}

	oldFiltersById := map[string]*gm.Filter{}
	filterQueryElems := map[string]*filter.FilterElement{}
	for _, fltr := range filters {
		oldFiltersById[fltr.Id] = fltr
		filterStr := fltr.Criteria.Query
		if filterStr != "" {
			filterElems, err := filter.ParseElement(filterStr)
			if err != nil {
				prnt.StderrLog.Fatalf("Error parsing filter \"%s\": %v\n", filterStr, err)
			}
			filterQueryElems[fltr.Id] = filterElems
		}
	}

	err = template.UpdateMetaGroups(filterQueryElems)
	if err != nil {
		prnt.StderrLog.Fatalln("Template error:", err)
	}

	var updatedFilters []*gm.Filter
	hasPrintedHeader := false

	// Print the diffs, and make copies of the Fitler objects
	for id, filterQElem := range filterQueryElems {
		oldFilter := oldFiltersById[id]
		newQuery := filterQElem.FullFilterStr()
		if newQuery != oldFilter.Criteria.Query {
			if !hasPrintedHeader {
				prnt.LPrintln(prnt.Quietable, prnt.Colorize("Updates to be done:", "bold"))
				hasPrintedHeader = true
			}
			updatedFilter := copyFilterAndCriteria(oldFilter)
			updatedFilter.Criteria.Query = newQuery
			updatedFilters = append(updatedFilters, updatedFilter)
			gHelper.PrintFilterDiff(oldFilter, updatedFilter)
			prnt.LPrintln(prnt.Quietable, "")
		}
	}

	maybeDoFilterChanges(gHelper, updatedFilters)
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
	Use:   "list [SEARCH_REGEXP]",
	Short: "List Gmail filters",
	Long: `List Gmail filters

Args:
SEARCH_REGEXP - Only show filters matching this pattern`,
	Aliases: []string{"ls"},
	Args:    cobra.MaximumNArgs(1),
	Run:     runListFilterCmd,
}

var replaceFilterCmd = &cobra.Command{
	Use:   "replace SEARCH_REGEXP SUB_STR",
	Short: "Replace parts of Gmail filters",
	Long: `Replace parts of Gmail filters

Args:
SEARCH_REGEXP - Pattern to use to replace field values
SUB_STR       - String to substitute into fields.
                $1,$2... will substitute to matched groups.`,
	Aliases: []string{"sub"},
	Args:    cobra.ExactArgs(2),
	Run:     runReplaceFilterCmd,
}

var updateFilterCmd = &cobra.Command{
	Use:   "update",
	Short: "Update Gmail filter templates",
	Run:   runUpdateFilterCmd,
}

func init() {
	// filter list
	filterCmd.AddCommand(listFilterCmd)

	// filter replace
	filterCmd.AddCommand(replaceFilterCmd)
	replaceFilterCmd.Flags().StringVarP(&replaceFilterField, "field", "f", "Query",
		"The criteria field to run the replace on.")
	addDryFlag(replaceFilterCmd)
	addAssumeYesFlag(replaceFilterCmd)

	// filter update
	filterCmd.AddCommand(updateFilterCmd)
	addDryFlag(updateFilterCmd)
	addAssumeYesFlag(updateFilterCmd)

	RootCmd.AddCommand(filterCmd)
}
