package cmd

import (
	"fmt"
	"log"

	gm "google.golang.org/api/gmail/v1"

	"github.com/spf13/cobra"

	"github.com/tsiemens/gmail-tools/api"
	"github.com/tsiemens/gmail-tools/config"
	"github.com/tsiemens/gmail-tools/prnt"
)

var DryRun = false
var AssumeYes = false

type MsgLabelModOptions struct {
	LabelNamesToAdd []string
	LabelNamesToRemove []string
	Touch           bool
	Trash           bool
	Archive         bool
}

var CmdMsgLabelModOptions = MsgLabelModOptions{
	LabelNamesToAdd: []string{},
	LabelNamesToRemove: []string{},
	Touch:           false,
	Trash:           false,
	Archive:         false,
}

func addDryFlag(command *cobra.Command) {
	command.Flags().BoolVarP(&DryRun, "dry", "n", false,
		"Perform no action, just print what would be done")
}

func addAssumeYesFlag(command *cobra.Command) {
	command.Flags().BoolVarP(&AssumeYes, "assumeyes", "y", false,
		"Answer 'yes' for all prompts")
}

// @targetDesc should be something describing what messages will be labelled.
func addLabelModFlags(command *cobra.Command) {
	command.Flags().StringArrayVar(
		&CmdMsgLabelModOptions.LabelNamesToAdd, "add-label", []string{},
		"Apply a label to messages (may be provided multiple times)")
	command.Flags().StringArrayVar(
		&CmdMsgLabelModOptions.LabelNamesToRemove, "rm-label", []string{},
			"Remove a label from messages (may be provided multiple times)")
	command.Flags().BoolVarP(&CmdMsgLabelModOptions.Touch, "touch", "t", false,
		"Apply 'touched' label from ~/.gmailcli/config.yaml")
	command.Flags().BoolVar(&CmdMsgLabelModOptions.Trash, "trash", false,
		"Send messages to the trash")
	command.Flags().BoolVar(&CmdMsgLabelModOptions.Archive, "archive", false,
		"Archive messages (remove from inbox)")
}

func ValidateTouchOption(conf *config.Config) {
	if CmdMsgLabelModOptions.Touch && conf.ApplyLabelOnTouch == "" {
		prnt.StderrLog.Fatalf("No ApplyLabelOnTouch property found in %s\n",
			conf.ConfigFile)
	}
}

func maybeApplyLabels(
	msgs []*gm.Message, gHelper *GmailHelper,
	labelsToAdd []api.Label, labelsToRemove []api.Label) {
	iterator := api.SizedMessageIdIteratorFromMsgs(msgs)
	maybeApplyLabelsToMsgIdIter(iterator, gHelper, labelsToAdd, labelsToRemove)
}

func maybeApplyLabelsToMsgIdIter(
	msgs api.SizedMessageIdIterator, gHelper *GmailHelper,
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
			err := gHelper.Msgs.ApplyLabelsByIdIter(msgs, labelsToAdd, labelsToRemove)
			if err != nil {
				log.Fatalf("Failed to %s: %s\n", actionStr, err)
			} else {
				prnt.HPrintln(prnt.Quietable, "Label(s) applied")
			}
		}
	}
}

func modifyMsgLabels(gHelper *GmailHelper, msgs []*gm.Message, opts *MsgLabelModOptions) {
	modifyMsgLabelsByMsgIdIter(
		gHelper, api.SizedMessageIdIteratorFromMsgs(msgs), opts)
}

func modifyMsgLabelsByMsgIdIter(
	gHelper *GmailHelper, msgIdIter api.SizedMessageIdIterator, opts *MsgLabelModOptions) {

	var labelsToAdd []api.Label = nil
	if len(opts.LabelNamesToAdd) > 0 {
		labelsToAdd = api.LabelsFromLabelNames(opts.LabelNamesToAdd)
	}
	var labelsToRemove []api.Label = nil
	if len(opts.LabelNamesToRemove) > 0 {
		labelsToRemove = api.LabelsFromLabelNames(opts.LabelNamesToRemove)
	}

	if opts.Archive {
		labelsToRemove = append(labelsToRemove, api.InboxLabel)
	}
	if opts.Touch {
		labelsToAdd = append(labelsToAdd, gHelper.GetTouchLabel())
	}
	if opts.Trash {
		labelsToAdd = append(labelsToAdd, api.TrashLabel)
	}

	if labelsToAdd != nil || labelsToRemove != nil {
		maybeApplyLabelsToMsgIdIter(msgIdIter, gHelper, labelsToAdd, labelsToRemove)
	}
}
