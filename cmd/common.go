package cmd

import "github.com/spf13/cobra"

var DryRun = false

func addDryFlag(command *cobra.Command) {
	command.Flags().BoolVarP(&DryRun, "dry", "n", false,
		"Perform no action, just print what would be done")
}
