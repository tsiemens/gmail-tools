package cmd

import (
	"fmt"
	"log"

	gm "google.golang.org/api/gmail/v1"

	"github.com/spf13/cobra"

	"github.com/tsiemens/gmail-tools/api"
	"github.com/tsiemens/gmail-tools/prnt"
)

var DryRun = false
var AssumeYes = false

func addDryFlag(command *cobra.Command) {
	command.Flags().BoolVarP(&DryRun, "dry", "n", false,
		"Perform no action, just print what would be done")
}

func addAssumeYesFlag(command *cobra.Command) {
	command.Flags().BoolVarP(&AssumeYes, "assumeyes", "y", false,
		"Answer 'yes' for all prompts")
}

func maybeApplyLabels(
	msgs []*gm.Message, gHelper *GmailHelper,
	labelsToAdd []api.Label, labelsToRemove []api.Label) {

	actionStr := ""
	if labelsToAdd != nil && len(labelsToAdd) > 0 {
		actionStr = actionStr + fmt.Sprintf("add label(s) %v", labelsToAdd)
	}
	if labelsToRemove != nil && len(labelsToRemove) > 0 {
		separator := ""
		if actionStr != "" {
			separator += ", "
		}
		actionStr = actionStr + separator + fmt.Sprintf("remove label(s) %v", labelsToRemove)
	}

	if DryRun {
		fmt.Printf("Skipping application of %s (--dry provided)\n", actionStr)
	} else {
		if MaybeConfirmFromInput(fmt.Sprintf("Apply: %s ?", actionStr), true) {
			err := gHelper.Msgs.ApplyLabels(msgs, labelsToAdd, labelsToRemove)
			if err != nil {
				log.Fatalf("Failed to %s: %s\n", actionStr, err)
			} else {
				prnt.HPrintln(prnt.Quietable, "Label(s) applied")
			}
		}
	}
}
