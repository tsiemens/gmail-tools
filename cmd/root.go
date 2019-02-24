package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	// "github.com/spf13/viper"

	"github.com/tsiemens/gmail-tools/api"
	"github.com/tsiemens/gmail-tools/prnt"
	"github.com/tsiemens/gmail-tools/util"
)

var Verbose = false
var Quiet = false
var BatchMode = false
var EmailToAssert string
var ClearCache = false

func MaybeConfirmFromInput(msg string, defaultVal bool) bool {
	if AssumeYes {
		return true
	}
	if BatchMode {
		return defaultVal
	}
	return util.ConfirmFromInput(msg, defaultVal)
}

func MaybeConfirmFromInputLong(msg string) bool {
	if AssumeYes {
		return true
	}
	if BatchMode {
		return false
	}
	return util.ConfirmFromInputLong(msg)
}

var cfgFile string

func cmdName() string {
	binName := os.Args[0]
	return filepath.Base(binName)
}

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   cmdName(),
	Short: "Cli tools for Gmail",
	Long: `A cli tool which can be used to perform more advanced operations on
a gmail account, via the provided Google APIs.`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	//	Run: func(cmd *cobra.Command, args []string) { },
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	util.RunCleanupHandlers()
}

func init() {
	cobra.OnInitialize(onInit)

	// Persistent flags, which are global to the app cli
	RootCmd.PersistentFlags().BoolVar(
		&util.DebugMode, "debug", false, "Enable debug tracing")

	RootCmd.PersistentFlags().BoolVarP(&Verbose, "verbose", "v", false,
		"Print verbose output")

	RootCmd.PersistentFlags().BoolVarP(&Quiet, "quiet", "q", false,
		"Don't print unecessary output")

	RootCmd.PersistentFlags().BoolVar(&BatchMode, "batch", false,
		"Run script in batch mode. This will automatically use the default for "+
			"any prompt, and will not print colors or extraneous output")

	RootCmd.PersistentFlags().StringVar(&EmailToAssert, "assert-email", "",
		"check that the authorized account matches this email address, before taking"+
			"any action")

	RootCmd.PersistentFlags().BoolVar(&ClearCache, "clear-cache", false,
		"Delete the cache files before running the command")
}

// onInit reads in config file and ENV variables if set, and performs global
// or common actions before running command functions.
func onInit() {
	prnt.NoHumanOnly = BatchMode
	prnt.DebugMode = util.DebugMode
	if Quiet && Verbose {
		prnt.StderrLog.Fatalln("--verbose and --quiet are mutually exclusive")
	}
	if Verbose {
		prnt.LevelEnabled = prnt.VerboseLevel
	} else if Quiet {
		prnt.LevelEnabled = prnt.AlwaysLevel
	}

	if ClearCache {
		cache := api.NewCache()
		defer cache.Close()
		cache.Clear()
	}

	// if cfgFile != "" {
	//	 // Use config file from the flag.
	//	 viper.SetConfigFile(cfgFile)
	// } else {
	//	 // Find home directory.
	// // homedir "github.com/mitchellh/go-homedir"
	//	 home, err := homedir.Dir()
	//	 if err != nil {
	//		fmt.Println(err)
	//		os.Exit(1)
	//	}

	//	 // Search config in home directory with name ".gmail-tools-dummy" (without extension).
	//	 viper.AddConfigPath(home)
	//	 viper.SetConfigName(".gmail-tools-dummy")
	// }

	// viper.AutomaticEnv() // read in environment variables that match

	// // If a config file is found, read it in.
	// if err := viper.ReadInConfig(); err == nil {
	//	 fmt.Println("Using config file:", viper.ConfigFileUsed())
	// }
}
