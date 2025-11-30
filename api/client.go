package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/gmail/v1"

	"github.com/tsiemens/gmail-tools/prnt"
	"github.com/tsiemens/gmail-tools/util"
)

const (
	CredentialsDirName   = ".credentials"
	ClientSecretFileName = "client_secret.json"

	// Just use the user that we have credentials for
	DefaultUser = "me"
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

var FiltersScope = &ScopeProfile{
	Scopes:   []string{gmail.GmailMetadataScope, gmail.GmailSettingsBasicScope},
	CredFile: "gmailcli_filters.json",
}

// tokenCacheFile generates credential file ~/.credentials/gmailcli.json
// It returns the generated credential filepath
func tokenCacheFile(scope *ScopeProfile) (string, error) {
	return util.HomeDirAndFile(CredentialsDirName, scope.CredFile)
}

func clientSecretFile() string {
	return util.RequiredHomeDirAndFile(util.UserAppDirName, ClientSecretFileName)
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

func DeleteCachedScopeToken(scope *ScopeProfile) {
	cacheFile, err := tokenCacheFile(scope)
	if err != nil {
		log.Fatalf("Unable to get path to cached credential file. %v", err)
	}
	err = os.Remove(cacheFile)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			log.Fatalf("Unable to delete %s: %v", cacheFile, err)
		}
	}
}

// Opens a browser window/tab with url.
// Returns true if the command executed successfully.
func openBrowserTab(url string) {
	var args []string
	switch runtime.GOOS {
	case "darwin":
		// Mac
		args = []string{"open"}
	case "windows":
		args = []string{"cmd", "/c", "start"}
	default:
		// Linux
		args = []string{"xdg-open"}
	}
	cmd := exec.Command(args[0], append(args[1:], url)...)
	err := cmd.Start()
	if err != nil {
		prnt.StderrLog.Printf("Error opening browser: %v\n", err)
	}
}

// Starts a very simple webserver in a new sudo'd subprocess on http port 80.
// The server simply checks for the 'code' query parameter in the request URL,
// or if one doesn't exist, shows a form to enter it.
// Exits after it gets a code and returns it from this function.
func startCodeFetcherWebserver() string {
	codeChan := make(chan string)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code != "" {
			fmt.Fprintf(w, "Authorization code received. You can close this window.")
			codeChan <- code
			return
		}

		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `<html><body>
			<form action="/" method="GET">
				Enter Authorization Code: <input type="text" name="code">
				<input type="submit" value="Submit">
			</form>
			</body></html>`)
	})

	server := &http.Server{Addr: ":8080"}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Could not start web server: %v", err)
		}
	}()

	code := <-codeChan
	server.Close()
	return code
}

// getTokenFromWeb uses Config to request a Token.
// It returns the retrieved Token.
func getTokenFromWeb(config *oauth2.Config) *oauth2.Token {
	var configCopy oauth2.Config = *config
	configCopy.RedirectURL = "http://localhost:8080"

	authURL := configCopy.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Println("A browser tab/window should open automatically to proceed with the " +
		"authentication process.")
	fmt.Printf("If the browser page does not open, paste the following link in your "+
		"browser:\n\n%v\n\n", authURL)
	// fmt.Printf("Paste Authorization code here: ")
	openBrowserTab(authURL)

	fmt.Println("Starting local endpoint at http://localhost:8080 "+
		"(the authentication flow from link above should redirect here automatically. "+
		"If not, open the link in your browser and enter the code by hand. "+
		"It should be in the URL the auth flow tries to redirect to).")

	code := startCodeFetcherWebserver()

	tok, err := configCopy.Exchange(oauth2.NoContext, code)
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
	secretFname := clientSecretFile()
	return ioutil.ReadFile(secretFname)
}

func styledClientSecretInstructions() string {
	bold := prnt.Style().Bold().Codes()
	normal := prnt.Style().Normal().Codes()

	return fmt.Sprintf("gmailcli requires a %spersonal%s API project in the Google developer console.\n" +
		"1. Go to %shttps://console.developers.google.com%s\n" +
		"2. Create a new project\n" +
		"3. Go to the Credential section in the new project\n" +
		"4. Click \"Create credentials\", and select OAuth client ID\n" +
		"5. Select type \"Other\" for the credential\n" +
		"6. Click the \"Download JSON\" button on the new credential.\n" +
		"7. Move the downloaded credential file to %s%s%s",
		bold, normal,
		bold, normal,
		bold, clientSecretFile(), normal,
	)
}

func NewGmailClient(scope *ScopeProfile) *gmail.Service {
	ctx := context.Background()

	secret, err := getClientSecret()
	if err != nil {
		util.ExternFatalf("%s: %v\n\n%s\n",
			prnt.Style().FgRed().Bold().On("Error reading client secret file"),
			err, styledClientSecretInstructions())
	}

	config, err := google.ConfigFromJSON(secret, scope.ScopesString())
	if err != nil {
		util.ExternFatalf("%s: %v\n\n%s\n",
			prnt.Style().FgRed().Bold().On(
				"Error: Unable to parse client secret file " + clientSecretFile()),
			err, styledClientSecretInstructions())
	}
	client := getClient(ctx, config, scope)

	srv, err := gmail.New(client)
	if err != nil {
		util.ExternFatalf("%s %v",
			prnt.Style().FgRed().Bold().On("Error creating Gmail client"), err)
	}
	return srv
}
