package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tsiemens/gmail-tools/api"
	gm "google.golang.org/api/gmail/v1"
	"google.golang.org/api/googleapi"
)

var showJSON, showThread bool
var showFormat string
var searchLabelIDs []string
var searchSpamTrash bool
var searchPageSize int64
var searchPageToken string
var accountJSON, labelsJSON bool

var machineClient = api.NewMachineClient

func writeJSON(w io.Writer, v interface{}) error { return json.NewEncoder(w).Encode(v) }

type machineError struct {
	Kind       string `json:"kind"`
	Message    string `json:"message"`
	HTTPStatus int    `json:"httpStatus,omitempty"`
}

func errorDetails(err error) machineError {
	e := machineError{Kind: "invalid_input", Message: err.Error()}
	var h *googleapi.Error
	var n net.Error
	if errors.As(err, &h) {
		e.HTTPStatus = h.Code
		e.Kind = "http"
		switch h.Code {
		case 404:
			e.Kind = "not_found"
		case 401, 403:
			e.Kind = "authentication"
		case 429:
			e.Kind = "throttled"
		}
	} else if errors.As(err, &n) {
		e.Kind = "network"
	}
	return e
}
func printCommandError(err error) {
	jsonMode := false
	for _, a := range os.Args[1:] {
		if a == "--json" || a == "--json=true" {
			jsonMode = true
		}
	}
	if jsonMode {
		_ = writeJSON(os.Stderr, map[string]interface{}{"error": errorDetails(err)})
	} else {
		fmt.Fprintln(os.Stderr, err)
	}
}
func checkedService() (*gm.Service, *gm.Profile, error) {
	s, e := machineClient()
	if e != nil {
		return nil, nil, e
	}
	p, e := s.Users.GetProfile(api.DefaultUser).Do()
	if e != nil {
		return nil, nil, e
	}
	if EmailToAssert != "" && p.EmailAddress != EmailToAssert {
		return nil, nil, fmt.Errorf("authorized account %s did not match %s", p.EmailAddress, EmailToAssert)
	}
	return s, p, nil
}
func runStructuredShow(cmd *cobra.Command, args []string) error {
	if showHeadersOnly || showBrief || showTouch {
		return fmt.Errorf("--json/--thread/--format cannot be combined with --brief, --headers-only or --touch")
	}
	if showFormat != "metadata" && showFormat != "full" {
		return fmt.Errorf("--format must be metadata or full")
	}
	if showRaw && (showThread || cmd.Flags().Changed("format")) {
		return fmt.Errorf("--raw cannot be combined with --thread or --format")
	}
	if !showJSON && cmd.Flags().Changed("format") {
		return fmt.Errorf("--format requires --json")
	}
	s, _, e := checkedService()
	if e != nil {
		return e
	}
	format := showFormat
	if showRaw {
		format = "raw"
	}
	if showThread {
		t, e := s.Users.Threads.Get(api.DefaultUser, args[0]).Format(format).Do()
		if e != nil {
			return e
		}
		if showJSON {
			return writeJSON(cmd.OutOrStdout(), t)
		}
		for _, m := range t.Messages {
			fmt.Fprintln(cmd.OutOrStdout(), "Message:", m.Id)
			if m.Payload != nil {
				for _, h := range m.Payload.Headers {
					fmt.Fprintf(cmd.OutOrStdout(), "%s: %s\n", h.Name, h.Value)
				}
			}
			for _, b := range api.GetMessageBody(m) {
				fmt.Fprintln(cmd.OutOrStdout(), b)
			}
		}
		return nil
	}
	m, e := s.Users.Messages.Get(api.DefaultUser, args[0]).Format(format).Do()
	if e != nil {
		return e
	}
	return writeJSON(cmd.OutOrStdout(), m)
}
func runMachineSearch(cmd *cobra.Command, args []string) error {
	paging := cmd.Flags().Changed("page-size") || cmd.Flags().Changed("page-token")
	if paging && !searchPrintJson {
		return fmt.Errorf("explicit paging requires --json")
	}
	if paging && (searchPageSize < 1 || searchPageSize > 500 || cmd.Flags().Changed("max")) {
		return fmt.Errorf("--page-size must be 1–500; paging cannot be combined with --max")
	}
	if searchInteresting || searchUninteresting || searchOutdated || len(searchLabelRegexps) > 0 || len(searchCustomFilterNames) > 0 || len(searchInverseCustomFilterNames) > 0 || searchDumpCustomFilters || searchShowSummary || CmdMsgLabelModOptions.Touch || CmdMsgLabelModOptions.Trash || CmdMsgLabelModOptions.Archive || len(CmdMsgLabelModOptions.LabelNamesToAdd) > 0 || len(CmdMsgLabelModOptions.LabelNamesToRemove) > 0 {
		return fmt.Errorf("machine search cannot be combined with local filters, summary, or mutations")
	}
	s, _, e := checkedService()
	if e != nil {
		return e
	}
	query := strings.Join(args, " ")
	msgs := make([]*gm.Message, 0)
	token := searchPageToken
	for {
		size := int64(500)
		if paging {
			size = searchPageSize
		} else if searchMaxMsgs > 0 && searchMaxMsgs-int64(len(msgs)) < size {
			size = searchMaxMsgs - int64(len(msgs))
		}
		r, e := s.Users.Messages.List(api.DefaultUser).Q(query).LabelIds(searchLabelIDs...).IncludeSpamTrash(searchSpamTrash).MaxResults(size).PageToken(token).Do()
		if e != nil {
			return e
		}
		msgs = append(msgs, r.Messages...)
		token = r.NextPageToken
		if paging || token == "" || (searchMaxMsgs > 0 && int64(len(msgs)) >= searchMaxMsgs) {
			break
		}
	}
	if paging {
		return writeJSON(cmd.OutOrStdout(), struct {
			Messages []*gm.Message `json:"messages"`
			Next     string        `json:"nextPageToken,omitempty"`
		}{msgs, token})
	}
	if searchPrintIdsOnly {
		if searchPrintJson {
			return writeJSON(cmd.OutOrStdout(), msgs)
		}
		for _, m := range msgs {
			fmt.Fprintf(cmd.OutOrStdout(), "%s,%s\n", m.Id, m.ThreadId)
		}
		return nil
	}
	// Preserve the existing search JSON schema, using metadata without plugins.
	labels, e := s.Users.Labels.List(api.DefaultUser).Do()
	if e != nil {
		return e
	}
	names := map[string]string{}
	for _, l := range labels.Labels {
		names[l.Id] = l.Name
	}
	result := make([]*MessageJson, 0, len(msgs))
	for _, m := range msgs {
		m, e = s.Users.Messages.Get(api.DefaultUser, m.Id).Format("metadata").Do()
		if e != nil {
			return e
		}
		v := &MessageJson{MessageId: m.Id, ThreadId: m.ThreadId, Timestamp: m.InternalDate}
		for _, id := range m.LabelIds {
			v.Labels = append(v.Labels, names[id])
		}
		if m.Payload != nil {
			for _, h := range m.Payload.Headers {
				switch strings.ToLower(h.Name) {
				case "subject":
					v.Subject = h.Value
				case "from":
					v.From = h.Value
				}
			}
		}
		result = append(result, v)
	}
	if searchPrintJson {
		return writeJSON(cmd.OutOrStdout(), result)
	}
	for _, m := range result {
		fmt.Fprintf(cmd.OutOrStdout(), "%s %s\n", m.MessageId, m.Subject)
	}
	return nil
}
func init() {
	account := &cobra.Command{Use: "account", Args: cobra.NoArgs, RunE: func(c *cobra.Command, a []string) error {
		_, p, e := checkedService()
		if e != nil {
			return e
		}
		if accountJSON {
			return writeJSON(c.OutOrStdout(), p)
		}
		fmt.Fprintln(c.OutOrStdout(), p.EmailAddress)
		return nil
	}}
	account.Flags().BoolVar(&accountJSON, "json", false, "Output JSON")
	labels := &cobra.Command{Use: "labels"}
	list := &cobra.Command{Use: "list", Args: cobra.NoArgs, RunE: func(c *cobra.Command, a []string) error {
		s, _, e := checkedService()
		if e != nil {
			return e
		}
		r, e := s.Users.Labels.List(api.DefaultUser).Do()
		if e != nil {
			return e
		}
		if r.Labels == nil {
			r.Labels = []*gm.Label{}
		}
		if labelsJSON {
			return writeJSON(c.OutOrStdout(), r.Labels)
		}
		for _, l := range r.Labels {
			fmt.Fprintf(c.OutOrStdout(), "%s\t%s\t%s\n", l.Id, l.Name, l.Type)
		}
		return nil
	}}
	list.Flags().BoolVar(&labelsJSON, "json", false, "Output JSON")
	labels.AddCommand(list)
	RootCmd.AddCommand(account, labels)
}
