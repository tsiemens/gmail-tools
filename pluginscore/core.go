package main

import (
	gm "google.golang.org/api/gmail/v1"

	"github.com/tsiemens/gmail-tools/api"
	"github.com/tsiemens/gmail-tools/config"
	"github.com/tsiemens/gmail-tools/plugin"
	"github.com/tsiemens/gmail-tools/prnt"
)

func matchesCategory(cat string, m *gm.Message, helper *api.MsgHelper) bool {
	conf := config.AppConfig()

	m, err := helper.GetMessage(m.Id, api.LabelsOnly)
	if err != nil {
		prnt.StderrLog.Println("core.matchesCategory error:", err)
		return false
	}

	// Interesting and uninteresting are special categories.
	// If we say something is uninteresting, but it ends up also being
	// interesting, interesting category will supersede uninsteresting.
	// This logic is driven by the app core logic.
	if cat == plugin.CategoryInteresting {
		for _, lId := range m.LabelIds {
			lName := helper.LabelName(lId)
			labelIsUninteresting := false
			for _, labRe := range conf.UninterLabelRegexps {
				idxSlice := labRe.FindStringIndex(lName)
				if idxSlice != nil {
					labelIsUninteresting = true
					break
				}
			}
			if labelIsUninteresting {
				// If the label is explicitly uninteresting, then the "interesting" label
				// patterns are not applied.
				continue
			}
			for _, labRe := range conf.InterLabelRegexps {
				idxSlice := labRe.FindStringIndex(lName)
				if idxSlice != nil {
					return true
				}
			}
		}
		return false
	} else if cat == plugin.CategoryUninteresting {
		prnt.Deb.Ln("finding uninteresting labels...")
		for _, lId := range m.LabelIds {
			lName := helper.LabelName(lId)
			for _, labRe := range conf.UninterLabelRegexps {
				idxSlice := labRe.FindStringIndex(lName)
				if idxSlice != nil {
					return true
				}
			}
		}
		return false
	}
	return false
}

func detailRequiredForCategory(string) api.MessageDetailLevel {
	return api.LabelsAndPayload
}

func builder() *plugin.Plugin {
	return &plugin.Plugin{
		Name:                      "Sample",
		MatchesCategory:           matchesCategory,
		DetailRequiredForCategory: detailRequiredForCategory,
	}
}

var Builder plugin.PluginBuilder = builder
