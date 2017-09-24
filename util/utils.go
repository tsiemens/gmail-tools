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
)

const (
	UserAppDirName = ".gmailcli"
)

func ConfirmFromInput(msg string) bool {
	fmt.Printf("%s [Ny]: ", msg)
	stdin := bufio.NewReader(os.Stdin)
	reply, err := stdin.ReadString('\n')
	if err != nil {
		log.Fatalf("Unable to read input: %v", err)
	}

	return strings.HasPrefix(strings.ToLower(reply), "y")
}

func HomeDirAndFile(dir, fname string) (string, error) {
	usr, err := user.Current()
	if err != nil {
		return "", err
	}
	dirPath := filepath.Join(usr.HomeDir, dir)
	os.MkdirAll(dirPath, 0700)
	return filepath.Join(dirPath, url.QueryEscape(fname)), err
}

func RequiredHomeDirAndFile(dir, fname string) string {
	fname, err := HomeDirAndFile(dir, fname)
	if err != nil {
		path := filepath.Join(dir, fname)
		log.Fatalf("Unable to get %s: %v", path, err)
	}
	return fname
}
