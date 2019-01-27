package cmd

import (
	"fmt"
	"log"

	gm "google.golang.org/api/gmail/v1"

	"github.com/spf13/cobra"

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
	msgs []*gm.Message, gHelper *GmailHelper, labelNames []string) {

	if DryRun {
		fmt.Printf("Skipping applying label(s) %v (--dry provided)\n",
			labelNames)
	} else {
		if MaybeConfirmFromInput(fmt.Sprintf("Apply label(s) %v?", labelNames), true) {
			err := gHelper.ApplyLabels(msgs, labelNames)
			if err != nil {
				log.Fatalf("Failed to apply label(s): %s\n", err)
			} else {
				prnt.HPrintln(prnt.Quietable, "Label(s) applied")
			}
		}
	}
}

func maybeTouchMessages(msgs []*gm.Message, helper *GmailHelper) {
	maybeApplyLabels(msgs, helper, []string{helper.conf.ApplyLabelOnTouch})
}

func maybeTrashMessages(msgs []*gm.Message, helper *GmailHelper) {
	maybeApplyLabels(msgs, helper, []string{"TRASH"})
}
