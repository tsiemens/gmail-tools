package cmd

import (
	"github.com/spf13/cobra"

	"github.com/tsiemens/gmail-tools/api"
	"github.com/tsiemens/gmail-tools/prnt"
)

var forceAuthorize bool = false

func runAuthCmd(cmd *cobra.Command, args []string) {
	profileName := "email"
	if len(args) >= 1 {
		profileName = args[0]
	}
	var profile *api.ScopeProfile
	switch profileName {
	case "email":
		profile = api.ModifyScope
	case "filters":
		profile = api.FiltersScope
	default:
		prnt.StderrLog.Fatalf("Invalid profile name '%s'", profileName)
	}

	if forceAuthorize {
		api.DeleteCachedScopeToken(profile)
	}
	_ = api.NewGmailClient(profile)
}

// authCmd represents the auth command
var authCmd = &cobra.Command{
	Use:   "authorize [email|filters]",
	Short: "Just set up the authentication of this tool with a Google account",
	Long: `Authenticates the client with your gmail account.
Attempts to open a browser window, where the user can obtain an auth code for gmailcli
to use to access that account. By default, it will do nothing if already authenticated.
Pass --force to re-authenticate.

There are multiple authentication profiles the app uses, to avoid accidental changes
to the account. Currently, these profiles are:
	email (AKA 'modify'): Used to modify, search emails. This is the default used by the autorize command.
	filters: Used to modify and organize gmail filter rules.`,
	Aliases: []string{"auth"},
	Run:     runAuthCmd,
	Args:    cobra.RangeArgs(0, 1),
}

func init() {
	authCmd.Flags().BoolVar(&forceAuthorize, "force", false,
		"Re-authorize even if previously authorized.")
	RootCmd.AddCommand(authCmd)
}
