package prnt

import (
	"fmt"
	"log"
	"os"
)

var StderrLog = log.New(os.Stderr, "", 0)

type PrintType int

const (
	Always PrintType = iota
	Quietable
	Verbose
	Debug
)

type PrintLevel int

const (
	AlwaysLevel PrintLevel = iota
	QuietableLevel
	VerboseLevel
)

var NoHumanOnly = false
var LevelEnabled PrintLevel = QuietableLevel
var DebugMode = false

func isEnabled(level PrintType, humanOnly bool) bool {
	if humanOnly && NoHumanOnly {
		return false
	} else if level == Debug && DebugMode {
		return true
	}
	return int(level) <= int(LevelEnabled)
}

func LHPrint(level PrintType, humanOnly bool, v ...interface{}) {
	if isEnabled(level, humanOnly) {
		fmt.Print(v...)
	}
}
func LHPrintf(level PrintType, humanOnly bool, format string, v ...interface{}) {
	if isEnabled(level, humanOnly) {
		fmt.Printf(format, v...)
	}
}
func LHPrintln(level PrintType, humanOnly bool, v ...interface{}) {
	if isEnabled(level, humanOnly) {
		fmt.Println(v...)
	}
}

func HPrint(level PrintType, v ...interface{}) {
	LHPrint(level, true, v...)
}
func HPrintf(level PrintType, format string, v ...interface{}) {
	LHPrintf(level, true, format, v...)
}
func HPrintln(level PrintType, v ...interface{}) {
	LHPrintln(level, true, v...)
}

func LPrint(level PrintType, v ...interface{}) {
	LHPrint(level, false, v...)
}
func LPrintf(level PrintType, format string, v ...interface{}) {
	LHPrintf(level, false, format, v...)
}
func LPrintln(level PrintType, v ...interface{}) {
	LHPrintln(level, false, v...)
}

func Print(v ...interface{}) {
	LPrint(Always, v...)
}
func Printf(format string, v ...interface{}) {
	LPrintf(Always, format, v...)
}
func Println(v ...interface{}) {
	LPrintln(Always, v...)
}

const (
	bold      = "\033[1m"
	resetC    = "\033[0m"
	fgRed     = "\033[31m"
	fgGreen   = "\033[32m"
	fgYellow  = "\033[33m"
	fgBlue    = "\033[34m"
	fgMagenta = "\033[35m"
	fgCyan    = "\033[36m"
)

var Colors map[string]string

func init() {
	Colors = map[string]string{
		"bold":    bold,
		"red":     fgRed,
		"green":   fgGreen,
		"yellow":  fgYellow,
		"blue":    fgBlue,
		"magenta": fgMagenta,
		"cyan":    fgCyan,
	}
}

func Reset() string {
	if !NoHumanOnly {
		return resetC
	}
	return ""
}

func Fg(colorKey string) string {
	if !NoHumanOnly {
		if color, ok := Colors[colorKey]; ok {
			return color
		}
		log.Fatalf("Color %s is invalid", colorKey)
	}
	return ""
}
