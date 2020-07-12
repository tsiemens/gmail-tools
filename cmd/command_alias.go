package cmd

import (
	"github.com/golang-collections/collections/stack"
	"github.com/spf13/cobra"

	"github.com/tsiemens/gmail-tools/aliasutil"
	"github.com/tsiemens/gmail-tools/config"
	"github.com/tsiemens/gmail-tools/prnt"
	"github.com/tsiemens/gmail-tools/util"
)

const (
	MaxAliasRecursion int = 10
)

var aliasStack = stack.New()

func makeAlias(aliasStr string, target string) *cobra.Command {
	return &cobra.Command{
		Use:                aliasStr,
		Short:              "User defined alias for " + target,
		DisableFlagParsing: true,
		Run: func(cmd *cobra.Command, args []string) {
			aliasStack.Push(aliasStr)
			if aliasStack.Len() > MaxAliasRecursion {
				prnt.StderrLog.Printf(
					"Max alias recursion exceeded (%d). Dumping command stack "+
						"(most recent first):\n",
					MaxAliasRecursion)
				for aliasStack.Len() > 1 {
					prnt.StderrLog.Printf("%s\n", aliasStack.Pop())
				}
				prnt.StderrLog.Fatalln(aliasStack.Pop())
			}

			targetArgs, err := aliasutil.CreateAliasArgs(args, target)
			if err != nil {
				prnt.StderrLog.Printf("Error building arguments for alias: %v\n", err)
				return
			}

			util.Debugln("Running", targetArgs)
			RootCmd.SetArgs(targetArgs)
			RootCmd.Execute()

			aliasStack.Pop()
		},
	}
}

func loadAliases() {
	cfg := config.AppConfig()

	util.Debugln("Aliases:")
	for aliasStr, target := range cfg.Aliases {
		util.Debugln(aliasStr, ":", target)
		var alias = makeAlias(aliasStr, target)
		RootCmd.AddCommand(alias)
	}
}

func init() {
	loadAliases()
}
