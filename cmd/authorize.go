package cmd

import (
	"github.com/spf13/cobra"

	"github.com/tsiemens/gmail-tools/api"
	"github.com/tsiemens/gmail-tools/config"
)

func runAuthCmd(cmd *cobra.Command, args []string) {
	conf := config.AppConfig()
	srv := api.NewGmailClient(api.ModifyScope)
	NewGmailHelper(srv, api.DefaultUser, conf)
}

// authCmd represents the auth command
var authCmd = &cobra.Command{
	Use:     "authorize",
	Short:   "Just set up the authentication of this tool with a Google account",
	Aliases: []string{"auth"},
	Run:     runAuthCmd,
}

func init() {
	RootCmd.AddCommand(authCmd)
}
