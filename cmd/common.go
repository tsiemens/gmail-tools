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

func maybeTouchMessages(msgs []*gm.Message, helper *GmailHelper) {
	plural := ""
	if len(msgs) > 1 {
		plural = "s"
	}

	if DryRun {
		fmt.Println("Skipping touching message" + plural + " (--dry provided)")
	} else {
		if MaybeConfirmFromInput("Mark message"+plural+" touched?", true) {
			err := helper.TouchMessages(msgs)
			if err != nil {
				log.Fatalf("Failed to touch message"+plural+": %s\n", err)
			} else {
				prnt.HPrintln(prnt.Quietable, "Message"+plural+" marked touched")
			}
		}
	}
}
