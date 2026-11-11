package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/tsiemens/gmail-tools/api"
	"github.com/tsiemens/gmail-tools/config"
	gm "google.golang.org/api/gmail/v1"
)

func runUpdateMsgsCmd(cmd *cobra.Command, args []string) error {
	msgIds := args
	if len(msgIds) == 0 {
		return fmt.Errorf("no message or thread IDs provided")
	}

	conf := config.AppConfig()
	ValidateTouchOption(conf)

	srv, client := api.NewGmailClientWithHTTPClient(api.ModifyScope)
	gHelper := NewGmailHelper(srv, api.DefaultUser, conf, client)
	gHelper.Msgs.SkipMessageChecks, _ = cmd.Flags().GetBool("skip-msg-checks")
	byThread, _ := cmd.Flags().GetBool("thread")
	if byThread {
		var err error
		msgIds, err = messageIDsForThreads(srv, args)
		if err != nil {
			return err
		}
		// These IDs came directly from Gmail; no separate existence checks are needed.
		gHelper.Msgs.SkipMessageChecks = true
	}

	msgIdIter := api.SizedMessageIdIteratorFromIds(msgIds)
	modifyMsgLabelsByMsgIdIter(gHelper, msgIdIter, &CmdMsgLabelModOptions)

	return nil
}

// Resolve all threads before making changes, and update shared messages only once.
func messageIDsForThreads(srv *gm.Service, threadIDs []string) ([]string, error) {
	var ids []string
	seenThreads := make(map[string]bool)
	seenMessages := make(map[string]bool)
	for _, id := range threadIDs {
		if seenThreads[id] {
			continue
		}
		seenThreads[id] = true
		thread, err := srv.Users.Threads.Get(api.DefaultUser, id).Format("minimal").Fields("messages/id").Do()
		if err != nil {
			return nil, fmt.Errorf("thread %s: %w", id, err)
		}
		for _, msg := range thread.Messages {
			if !seenMessages[msg.Id] {
				ids = append(ids, msg.Id)
				seenMessages[msg.Id] = true
			}
		}
	}
	return ids, nil
}

var updateMsgsCmd = &cobra.Command{
	Use:     "update-msgs MSG_IDS...",
	Short:   "Updates messages based on ID",
	Long:    "Updates messages based on message IDs, or all messages in the supplied thread IDs with --thread.",
	Aliases: []string{},
	RunE:    runUpdateMsgsCmd,
	Args:    cobra.MinimumNArgs(1),
}

func init() {
	RootCmd.AddCommand(updateMsgsCmd)

	updateMsgsCmd.Flags().Bool("thread", false, "Treat arguments as thread IDs and update all their messages without separate message ID checks")
	updateMsgsCmd.Flags().Bool("skip-msg-checks", false, "Skip message ID validation (Gmail may silently ignore missing IDs)")
	addLabelModFlags(updateMsgsCmd)
	addDryFlag(updateMsgsCmd)
	addAssumeYesFlag(updateMsgsCmd)
}
