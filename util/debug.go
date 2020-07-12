package util

import (
	"log"
	"os"
)

var DebugMode = false
var DebugEnv = false

func DebugModeEnabled() bool {
	return DebugMode || DebugEnv
}

func Debug(v ...interface{}) {
	if DebugModeEnabled() {
		log.Print(v...)
	}
}
func Debugf(format string, v ...interface{}) {
	if DebugModeEnabled() {
		log.Printf(format, v...)
	}
}
func Debugln(v ...interface{}) {
	if DebugModeEnabled() {
		log.Println(v...)
	}
}

func init() {
	if len(os.Getenv("DEBUG")) > 0 {
		DebugEnv = true
	}
}
