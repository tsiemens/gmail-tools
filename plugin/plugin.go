package plugin

import (
	"fmt"
	"path/filepath"
	pluginlib "plugin"

	gm "google.golang.org/api/gmail/v1"

	"github.com/tsiemens/gmail-tools/api"
	"github.com/tsiemens/gmail-tools/prnt"
	"github.com/tsiemens/gmail-tools/util"
)

const (
	pluginDir = "plugins"
)

type InterestLevel int

// When combining different plugins' interest categorization, we will use these
// rules (with order preference):
// 1. any( strongly interesting ) -> stronly interesting
// 2. not 1. and any( stronly uninteresting ) -> stronly uninteresting
// 3. not the above and any( weakly interesting ) -> weakly intereting
// 4. Not the above and any( weakly uninteresting ) -> weakly uninteresting
// 5. all no opinion -> no opinion
const (
	// Used for specific message patterns which determine interest.
	StronglyInteresting InterestLevel = iota
	// Used for specific message patterns which determine disinterest.
	StronglyUninteresting
	// Interest determined via a heuristic
	WeaklyInteresting
	// Disinterest determined via a heuristic
	WeaklyUninteresting
	UnknownInterest
)

func (i1 InterestLevel) Combine(i2 InterestLevel) InterestLevel {
	if int(i1) < int(i2) {
		return i1
	}
	return i2
}

// Favours the most uncertain answer
func (i1 InterestLevel) InverseCombine(i2 InterestLevel) InterestLevel {
	if int(i1) > int(i2) {
		return i1
	}
	return i2
}

type MessageFilter struct {
	Desc    string
	Matches func(*gm.Message, *api.MsgHelper) bool
}

type Plugin struct {
	Name string

	MessageInterest           func(*gm.Message, *api.MsgHelper) InterestLevel
	DetailRequiredForInterest func() api.MessageDetailLevel

	OutdatedMessages func(string, *api.MsgHelper) []*gm.Message

	PrintMessageSummary func([]*gm.Message, *api.MsgHelper)

	MessageFilters map[string]*MessageFilter
}

type PluginBuilder func() *Plugin

func LoadPlugins() []*Plugin {
	dirName := filepath.Join(util.UserAppDirName, pluginDir)
	dirName = util.RequiredHomeBasedDir(dirName)

	files, err := filepath.Glob(filepath.Join(dirName, "*"))
	if err != nil {
		fmt.Println(err)
		prnt.StderrLog.Printf("Failed to retrieve plugin list: %s\n", err)
	}

	loadedPlugins := make([]*Plugin, 0)

	prnt.LPrintln(prnt.Debug, "debug: Loading plugins:")
	for _, file := range files {
		prnt.LPrintln(prnt.Debug, "debug:", file)
		plg, err := pluginlib.Open(file)
		if err != nil {
			prnt.StderrLog.Printf("Error loading plugin %s: %s\n", file, err)
			continue
		}

		builderSym, err := plg.Lookup("Builder")
		if err != nil {
			prnt.StderrLog.Printf("Error loading plugin %s: %s\n", file, err)
			continue
		}

		builderPtr, ok := builderSym.(*PluginBuilder)
		if !ok {
			prnt.StderrLog.Printf(
				"Error loading plugin %s: Builder was of type PluginBuilder\n", file)
			continue
		}

		loadedPlugins = append(loadedPlugins, (*builderPtr)())
	}

	return loadedPlugins
}
