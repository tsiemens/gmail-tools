package api

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/gmail/v1"
)

const (
	CredentialsDirName   = ".credentials"
	ClientSecretDirName  = ".gmailcli"
	ClientSecretFileName = "client_secret.json"
)

type ScopeProfile struct {
	Scopes   []string
	CredFile string
}

func (s *ScopeProfile) ScopesString() string {
	return strings.Join(s.Scopes, " ")
}

// Scope docs: https://godoc.org/google.golang.org/api/gmail/v1
// If modifying these scopes, delete your previously saved credentials
// at ~/.credentials/...
// TODO these maybe can share the same file. Need to test once I do some write actions
var ReadScope = &ScopeProfile{
	Scopes:   []string{gmail.GmailReadonlyScope},
	CredFile: "gmailcli_read.json",
}

var LabelsScope = &ScopeProfile{
	Scopes:   []string{gmail.GmailReadonlyScope, gmail.GmailMetadataScope},
	CredFile: "gmailcli_labels.json",
}

var ModifyScope = &ScopeProfile{
	Scopes:   []string{gmail.GmailModifyScope},
	CredFile: "gmailcli_modify.json",
}

func homeDirAndFile(dir, fname string) (string, error) {
	usr, err := user.Current()
	if err != nil {
		return "", err
	}
	tokenCacheDir := filepath.Join(usr.HomeDir, dir)
	os.MkdirAll(tokenCacheDir, 0700)
	return filepath.Join(tokenCacheDir, url.QueryEscape(fname)), err
}

// tokenCacheFile generates credential file ~/.credentials/gmailcli.json
// It returns the generated credential filepath
func tokenCacheFile(scope *ScopeProfile) (string, error) {
	return homeDirAndFile(CredentialsDirName, scope.CredFile)
}

func clientSecretFile() (string, error) {
	return homeDirAndFile(ClientSecretDirName, ClientSecretFileName)
}

// getClient uses a Context and Config to retrieve a Token
// then generate a Client. It returns the generated Client.
func getClient(ctx context.Context, config *oauth2.Config, scope *ScopeProfile) *http.Client {
	cacheFile, err := tokenCacheFile(scope)
	if err != nil {
		log.Fatalf("Unable to get path to cached credential file. %v", err)
	}
	tok, err := tokenFromFile(cacheFile)
	if err != nil {
		tok = getTokenFromWeb(config)
		saveToken(cacheFile, tok)
	}
	return config.Client(ctx, tok)
}

// getTokenFromWeb uses Config to request a Token.
// It returns the retrieved Token.
func getTokenFromWeb(config *oauth2.Config) *oauth2.Token {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Go to the following link in your browser then type the "+
		"authorization code: \n%v\n", authURL)

	var code string
	if _, err := fmt.Scan(&code); err != nil {
		log.Fatalf("Unable to read authorization code %v", err)
	}

	tok, err := config.Exchange(oauth2.NoContext, code)
	if err != nil {
		log.Fatalf("Unable to retrieve token from web %v", err)
	}
	return tok
}

// tokenFromFile retrieves a Token from a given file path.
// It returns the retrieved Token and any read error encountered.
func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	t := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(t)
	defer f.Close()
	return t, err
}

// saveToken uses a file path to create a file and store the
// token in it.
func saveToken(file string, token *oauth2.Token) {
	fmt.Printf("Saving credential file to: %s\n", file)
	f, err := os.OpenFile(file, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		log.Fatalf("Unable to cache oauth token: %v", err)
	}
	defer f.Close()
	json.NewEncoder(f).Encode(token)
}

func getClientSecret() ([]byte, error) {
	secretFname, err := clientSecretFile()
	if err != nil {
		log.Fatalf("Unable to get path to cached credential file. %v", err)
	}
	return ioutil.ReadFile(secretFname)
}

func NewGmailClient(scope *ScopeProfile) *gmail.Service {
	ctx := context.Background()

	secret, err := getClientSecret()
	if err != nil {
		log.Fatalf("Unable to read client secret file: %v", err)
	}

	config, err := google.ConfigFromJSON(secret, scope.ScopesString())
	if err != nil {
		log.Fatalf("Unable to parse client secret file to config: %v", err)
	}
	client := getClient(ctx, config, scope)

	srv, err := gmail.New(client)
	if err != nil {
		log.Fatalf("Unable to retrieve gmail Client %v", err)
	}
	return srv
}
