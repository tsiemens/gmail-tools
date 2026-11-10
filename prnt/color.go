package prnt

import (
	"os"

	"golang.org/x/term"
)

/*
TODO create color lib for use like so:


Println(prnt.Style().Bold().FgRed().On("This is bold red text\n"))


*/

type Styler struct {
	stylingString string
	enabled       bool
}

// IsTty returns true when stdout is a terminal.
func IsTty() bool {
	return term.IsTerminal(int(os.Stdout.Fd()))
}

func Style() *Styler {
	return &Styler{
		stylingString: "",
		enabled:       IsTty() && os.Getenv("NO_COLOR") == "" && os.Getenv("TERM") != "dumb",
	}
}

func (s *Styler) apply(code string) *Styler {
	s.stylingString += code
	return s
}

/**
 * Bold applies bold styling.
 */
func (s *Styler) Bold() *Styler {
	return s.apply("\033[1m")
}

/**
 * FgRed applies red foreground color.
 */
func (s *Styler) FgRed() *Styler {
	return s.apply("\033[31m")
}

/**
 * FgGreen applies green foreground color.
 */
func (s *Styler) FgGreen() *Styler {
	return s.apply("\033[32m")
}

/**
 * FgYellow applies yellow foreground color.
 */
func (s *Styler) FgYellow() *Styler {
	return s.apply("\033[33m")
}

/**
 * FgCyan applies cyan foreground color.
 */
func (s *Styler) FgCyan() *Styler {
	return s.apply("\033[36m")
}

/**
 * FgMagenta applies magenta foreground color.
 */
func (s *Styler) FgMagenta() *Styler {
	return s.apply("\033[35m")
}

func (s *Styler) Normal() *Styler {
	return s.apply("\033[0m")
}

/**
 * Codes returns the styling string.
 */
func (s *Styler) Codes() string {
	if s.enabled {
		return s.stylingString
	}
	return ""
}

/**
 * On applies the styling to the given string, if enabled.
 */
func (s *Styler) On(str string) string {
	if s.enabled {
		return s.stylingString + str + "\033[0m"
	}
	return str
}

// Dim renders secondary metadata with reduced emphasis.
func (s *Styler) Dim() *Styler { return s.apply("\033[2m") }

// ANSI applies an existing configured ANSI style, respecting color detection.
func (s *Styler) ANSI(code string) *Styler { return s.apply(code) }
