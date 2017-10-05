package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	// "github.com/spf13/viper"

	"github.com/tsiemens/gmail-tools/util"
)

var Verbose = false
var BatchMode = false

func MaybeConfirmFromInput(msg string, defaultVal bool) bool {
	if BatchMode {
		return defaultVal
	}
	return util.ConfirmFromInput(msg, defaultVal)
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
}

func init() {
	cobra.OnInitialize(initConfig)

	// Persistent flags, which are global to the app cli
	RootCmd.PersistentFlags().BoolVar(
		&util.DebugMode, "debug", false, "Enable debug tracing")

	RootCmd.PersistentFlags().BoolVarP(&Verbose, "verbose", "v", false,
		"Print verbose output")

	RootCmd.PersistentFlags().BoolVarP(&BatchMode, "batch", "b", false,
		"Run script in batch mode. This will automatically use the default for "+
			"any prompt, and will not print colors or extraneous output")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	// RootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
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
