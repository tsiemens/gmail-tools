package util

import (
	"log"
)

var DebugMode = false

func Debug(v ...interface{}) {
	if DebugMode {
		log.Print(v...)
	}
}
func Debugf(format string, v ...interface{}) {
	if DebugMode {
		log.Printf(format, v...)
	}
}
func Debugln(v ...interface{}) {
	if DebugMode {
		log.Println(v...)
	}
}
