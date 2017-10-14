package util

import (
	"runtime/debug"

	"github.com/tsiemens/gmail-tools/prnt"
)

// Can be set by tests if they want to catch asserts
var AssertsPanic bool = false

func Assert(cond bool, o ...interface{}) {
	if !cond {
		if AssertsPanic {
			prnt.StderrLog.Panic(o...)
		} else {
			debug.PrintStack()
			prnt.StderrLog.Fatal(o...)
		}
	}
}

func Assertf(cond bool, fmt string, o ...interface{}) {
	if !cond {
		if AssertsPanic {
			prnt.StderrLog.Panicf(fmt, o...)
		} else {
			debug.PrintStack()
			prnt.StderrLog.Fatalf(fmt, o...)
		}
	}
}
