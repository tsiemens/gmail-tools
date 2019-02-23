package prnt

import (
	"fmt"
	"log"
	"os"
	"strings"
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

type Printer struct {
	level     PrintType
	humanOnly bool
}

func (p *Printer) P(v ...interface{}) {
	LHPrint(p.level, p.humanOnly, v...)
}
func (p *Printer) F(format string, v ...interface{}) {
	LHPrintf(p.level, p.humanOnly, format, v...)
}

func (p *Printer) Ln(v ...interface{}) {
	LHPrintln(p.level, p.humanOnly, v...)
}

type PrinterWrapper struct {
	Always  Printer
	Quiet   Printer
	Verbose Printer
	Debug   Printer
}

var Hum PrinterWrapper
var Quiet Printer
var Verb Printer
var Deb Printer

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
	Hum = PrinterWrapper{
		Always:  Printer{Always, true},
		Quiet:   Printer{Quietable, true},
		Verbose: Printer{Verbose, true},
		Debug:   Printer{Debug, true},
	}
	Quiet = Printer{Quietable, false}
	Verb = Printer{Verbose, false}
	Deb = Printer{Debug, false}

	Colors = map[string]string{
		"bold":    bold,
		"reset":   resetC,
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

func Colorize(str, colorKey string) string {
	return Fg(colorKey) + str + Fg("reset")
}

type ProgressPrinter struct {
	total       int
	current     int
	previousLen int
}

func NewProgressPrinter(total int) *ProgressPrinter {
	return &ProgressPrinter{total: total, current: 0, previousLen: 0}
}

// n: The number of units to increment.
func (p *ProgressPrinter) Progress(n int) {
	if p.previousLen > 0 {
		// Reset previous
		Hum.Always.P(strings.Repeat("\x08", p.previousLen))
	}

	p.current += n
	progressStr := fmt.Sprintf("%d/%d", p.current, p.total)
	p.previousLen = len(progressStr)
	Hum.Always.P(progressStr)
}
