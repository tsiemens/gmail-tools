package main

import "fmt"
import "github.com/tsiemens/gmail-tools/plugin"

func doTest() {
	fmt.Println("plugin sample test")
}

func builder() *plugin.Plugin {
	return &plugin.Plugin{
		Name: "Sample",
		Test: doTest,
	}
}

var Builder plugin.PluginBuilder = builder
