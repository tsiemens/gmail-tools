package main

import (
	gm "google.golang.org/api/gmail/v1"

	"github.com/tsiemens/gmail-tools/api"
	"github.com/tsiemens/gmail-tools/plugin"
)

func matchesCategory(string, *gm.Message, *api.MsgHelper) bool {
	return false
}

func builder() *plugin.Plugin {
	return &plugin.Plugin{
		Name:            "Sample",
		MatchesCategory: matchesCategory,
	}
}

var Builder plugin.PluginBuilder = builder
