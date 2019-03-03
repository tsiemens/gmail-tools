package main

import (
	gm "google.golang.org/api/gmail/v1"

	"github.com/tsiemens/gmail-tools/api"
	"github.com/tsiemens/gmail-tools/config"
	"github.com/tsiemens/gmail-tools/plugin"
	"github.com/tsiemens/gmail-tools/prnt"
)

func messageInterest(m *gm.Message, helper *api.MsgHelper) plugin.InterestLevel {
	conf := config.AppConfig()

	m, err := helper.GetMessage(m.Id, api.LabelsOnly)
	if err != nil {
		prnt.StderrLog.Println("core.messageInterest error:", err)
		return plugin.UnknownInterest
	}
	threadLabelNames, err := helper.ThreadLabelNames(m.ThreadId)
	if err != nil {
		prnt.StderrLog.Println("core.messageInterest error:", err)
		return plugin.UnknownInterest
	}

	// Go through labels and determine their interest.
	// If any label is AlwaysUninteresting, then immediately classify as
	// StronglyUninteresting. This is generally reserved for labels manually
	// applied, similar to the mute label.
	for _, lName := range threadLabelNames {
		for _, labRe := range conf.AlwaysUninterLabelRegexps {
			idxSlice := labRe.FindStringIndex(lName)
			if idxSlice != nil {
				prnt.Deb.Ln("label matched always uninteresting pattern", labRe)
				return plugin.StronglyUninteresting
			}
		}
	}

	// For each label, if it is marked uninteresting, this overrides patterns marking
	// interest.
	// If any label is marked interesting however, then the message as a whole
	// is categorized as interesting.
	matchedUninteresting := false
	for _, lName := range threadLabelNames {
		labelIsUninteresting := false
		for _, labRe := range conf.UninterLabelRegexps {
			idxSlice := labRe.FindStringIndex(lName)
			if idxSlice != nil {
				prnt.Deb.Ln("label matched uninteresting pattern", labRe)
				labelIsUninteresting = true
				break
			}
		}
		if labelIsUninteresting {
			// If the label is explicitly uninteresting, then the "interesting" label
			// patterns are not applied.
			matchedUninteresting = true
			continue
		}
		for _, labRe := range conf.InterLabelRegexps {
			idxSlice := labRe.FindStringIndex(lName)
			if idxSlice != nil {
				prnt.Deb.Ln("label matched interesting pattern", labRe)
				return plugin.WeaklyInteresting
			}
		}
	}

	if matchedUninteresting {
		return plugin.WeaklyUninteresting
	}
	return plugin.UnknownInterest
}

func detailRequiredForInterest() api.MessageDetailLevel {
	return api.LabelsOnly
}

func builder() *plugin.Plugin {
	return &plugin.Plugin{
		Name:                      "Core",
		MessageInterest:           messageInterest,
		DetailRequiredForInterest: detailRequiredForInterest,
	}
}

var Builder plugin.PluginBuilder = builder
