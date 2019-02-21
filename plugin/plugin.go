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

// Common categories
const (
	CategoryInteresting   = "interesting"
	CategoryUninteresting = "uninteresting"
)

type Plugin struct {
	Name string

	MatchesCategory func(string, *gm.Message, *api.MsgHelper) bool
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
