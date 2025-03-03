package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/tsiemens/gmail-tools/api"
	"github.com/tsiemens/gmail-tools/config"
)

func runUpdateMsgsCmd(cmd *cobra.Command, args []string) error {
	msgIds := args
	if len(msgIds) == 0 {
		fmt.Errorf("No message IDs provided")
	}

	conf := config.AppConfig()
	ValidateTouchOption(conf)

	srv := api.NewGmailClient(api.ModifyScope)
	gHelper := NewGmailHelper(srv, api.DefaultUser, conf)

	msgIdIter := api.SizedMessageIdIteratorFromIds(msgIds)
	modifyMsgLabelsByMsgIdIter(gHelper, msgIdIter, &CmdMsgLabelModOptions)

	return nil
}

var updateMsgsCmd = &cobra.Command{
	Use:     "update-msgs MSG_IDS...",
	Short:   "Updates messages based on ID",
	Aliases: []string{},
	RunE:    runUpdateMsgsCmd,
	Args:    cobra.MinimumNArgs(1),
}

func init() {
	RootCmd.AddCommand(updateMsgsCmd)

	addLabelModFlags(updateMsgsCmd)
	addDryFlag(updateMsgsCmd)
	addAssumeYesFlag(updateMsgsCmd)
}
