package main

import (
	"fmt"
	"log"

	"github.com/tsiemens/gmail-tools/api"
)

func main() {
	srv := api.NewGmailClient()

	// Temporary demo script
	user := "me"
	r, err := srv.Users.Labels.List(user).Do()
	if err != nil {
		log.Fatalf("Unable to retrieve labels. %v", err)
	}
	if len(r.Labels) > 0 {
		fmt.Print("Labels:\n")
		for _, l := range r.Labels {
			fmt.Printf("- %s\n", l.Name)
		}
	} else {
		fmt.Print("No labels found.")
	}

}
