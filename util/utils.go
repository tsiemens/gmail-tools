package util

import (
	"bufio"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/tsiemens/gmail-tools/prnt"
)

const (
	UserAppDirName = ".gmailcli"

	Bold      = "\033[1m"
	ResetC    = "\033[0m"
	FgRed     = "\033[31m"
	FgGreen   = "\033[32m"
	FgYellow  = "\033[33m"
	FgBlue    = "\033[34m"
	FgMagenta = "\033[35m"
	FgCyan    = "\033[36m"
)

var Colors map[string]string

func init() {
	Colors = map[string]string{
		"bold":    Bold,
		"red":     FgRed,
		"green":   FgGreen,
		"yellow":  FgYellow,
		"blue":    FgBlue,
		"magenta": FgMagenta,
		"cyan":    FgCyan,
	}
}

func ConfirmFromInput(msg string, defaultYes bool) bool {
	defaultStr := "[y/N]"
	if defaultYes {
		defaultStr = "[Y/n]"
	}
	fmt.Printf("%s %s: ", msg, defaultStr)
	stdin := bufio.NewReader(os.Stdin)
	reply, err := stdin.ReadString('\n')
	if err != nil {
		log.Fatalf("Unable to read input: %v", err)
	}

	if defaultYes {
		return !strings.HasPrefix(strings.ToLower(reply), "n")
	} else {
		return strings.HasPrefix(strings.ToLower(reply), "y")
	}
}

func ConfirmFromInputLong(msg string) bool {
	for {
		fmt.Printf("%s [No/yes]: ", msg)
		stdin := bufio.NewReader(os.Stdin)
		reply, err := stdin.ReadString('\n')
		if err != nil {
			log.Fatalf("Unable to read input: %v", err)
		}

		if strings.HasPrefix(strings.ToLower(reply), "y") {
			if strings.TrimSpace(strings.ToLower(reply)) == "yes" {
				return true
			} else {
				fmt.Println("Whole word 'yes' is required")
			}
		} else {
			return false
		}
	}
}

// Create and return the name of a directory at $HOME/<dir>
func HomeBasedDir(dir string) (string, error) {
	usr, err := user.Current()
	if err != nil {
		return "", err
	}
	dirPath := filepath.Join(usr.HomeDir, dir)
	os.MkdirAll(dirPath, 0700)
	return dirPath, nil
}

func HomeDirAndFile(dir, fname string) (string, error) {
	dirPath, err := HomeBasedDir(dir)
	if err != nil {
		return "", err
	}
	return filepath.Join(dirPath, url.QueryEscape(fname)), nil
}

func RequiredHomeDirAndFile(dir, fname string) string {
	fname, err := HomeDirAndFile(dir, fname)
	if err != nil {
		path := filepath.Join(dir, fname)
		log.Fatalf("Unable to get %s: %v", path, err)
	}
	return fname
}

func RequiredHomeBasedDir(dir string) string {
	dirName, err := HomeBasedDir(dir)
	if err != nil {
		path := filepath.Join(dir, dirName)
		log.Fatalf("Unable to get %s/: %v", path, err)
	}
	return dirName
}

func CheckErr(err error, v ...interface{}) {
	if err != nil {
		v = append(v, err)
		prnt.StderrLog.Fatalln(v...)
	}
}
func CheckErrf(err error, format string, v ...interface{}) {
	if err != nil {
		v = append(v, err)
		prnt.StderrLog.Fatalf(format+" %v\n", v...)
	}
}
