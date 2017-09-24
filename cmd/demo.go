package cmd

import (
	"fmt"
	"log"

	"github.com/spf13/cobra"

	"github.com/tsiemens/gmail-tools/api"
)

func runDemoCmd(cmd *cobra.Command, args []string) {
	srv := api.NewGmailClient(api.ReadScope)

	// Temporary demo script
	user := "me"
	r, err := srv.Users.Labels.List(user).Do()
	if err != nil {
		log.Fatalf("Unable to retrieve labels. %v", err)
	}
	if len(r.Labels) > 0 {
		fmt.Print("Labels:\n")
		for _, l := range r.Labels {
			fmt.Printf("- %s: %s\n", l.Id, l.Name)
		}
	} else {
		fmt.Print("No labels found.")
	}
}

// demoCmd represents the demo command
var demoCmd = &cobra.Command{
	Use:   "demo",
	Short: "Runs a simple demo of the Gmail API",
	Run:   runDemoCmd,
}

func init() {
	RootCmd.AddCommand(demoCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// demoCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// demoCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
