package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/tsiemens/gmail-tools/api"
	"github.com/tsiemens/gmail-tools/plugin"
	"github.com/tsiemens/gmail-tools/prnt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	gm "google.golang.org/api/gmail/v1"
)

type handlerTransport struct{ handler http.HandlerFunc }

func (h handlerTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	w := httptest.NewRecorder()
	h.handler(w, r)
	return w.Result(), nil
}
func fakeMachine(t *testing.T, handler http.HandlerFunc) {
	t.Helper()
	old := machineClient
	machineClient = func() (*gm.Service, error) {
		s, e := gm.New(&http.Client{Transport: handlerTransport{handler}})
		if e == nil {
			s.BasePath = "https://gmail.test/"
		}
		return s, e
	}
	t.Cleanup(func() { machineClient = old })
}
func TestStructuredThreadFresh(t *testing.T) {
	oldJSON, oldThread := showJSON, showThread
	oldFormat := showFormat
	defer func() { showJSON, showThread, showFormat = oldJSON, oldThread, oldFormat }()
	showJSON, showThread, showFormat = true, true, "metadata"
	calls := 0
	fakeMachine(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("mutation %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/gmail/v1/users/me/profile" {
			fmt.Fprint(w, `{"emailAddress":"a@b"}`)
			return
		}
		if r.URL.Path != "/gmail/v1/users/me/threads/t" || r.URL.Query().Get("format") != "metadata" {
			t.Errorf("request %s", r.URL)
		}
		calls++
		if calls == 1 {
			fmt.Fprint(w, `{"id":"t","messages":[{"id":"a","internalDate":"12"}]}`)
		} else {
			fmt.Fprint(w, `{"id":"t","messages":[{"id":"a","internalDate":"12"},{"id":"b","internalDate":"12"}]}`)
		}
	})
	c := &cobra.Command{}
	var b bytes.Buffer
	c.SetOut(&b)
	if e := runStructuredShow(c, []string{"t"}); e != nil {
		t.Fatal(e)
	}
	b.Reset()
	if e := runStructuredShow(c, []string{"t"}); e != nil {
		t.Fatal(e)
	}
	if !bytes.Contains(b.Bytes(), []byte(`"id":"b"`)) {
		t.Fatal(b.String())
	}
}
func TestMachineSearchLaterPageFailure(t *testing.T) {
	old := searchPrintJson
	searchPrintJson = true
	defer func() { searchPrintJson = old }()
	fakeMachine(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/gmail/v1/users/me/profile" {
			fmt.Fprint(w, `{"emailAddress":"a@b"}`)
			return
		}
		if r.URL.Query().Get("pageToken") == "" {
			fmt.Fprint(w, `{"messages":[{"id":"a","threadId":"t"}],"nextPageToken":"next"}`)
			return
		}
		w.WriteHeader(429)
		fmt.Fprint(w, `{"error":{"code":429,"message":"slow down"}}`)
	})
	c := &cobra.Command{}
	var b bytes.Buffer
	c.SetOut(&b)
	e := runMachineSearch(c, nil)
	if e == nil || errorDetails(e).HTTPStatus != 429 || b.Len() != 0 {
		t.Fatalf("err %v output %s", e, b.String())
	}
}
func TestAccountAssertionBeforeAccess(t *testing.T) {
	old := EmailToAssert
	EmailToAssert = "other"
	defer func() { EmailToAssert = old }()
	fakeMachine(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/gmail/v1/users/me/profile" {
			t.Fatalf("unexpected access %s", r.URL)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"emailAddress":"a@b"}`)
	})
	if _, _, e := checkedService(); e == nil {
		t.Fatal("expected mismatch")
	}
}

// Exercise the command dispatcher, Gmail pagination, local filters and both JSON
// schemas together, without credentials or installed plugin binaries.
func TestJSONSearchLocalFilters(t *testing.T) {
	oldJSON, oldIDs := searchPrintJson, searchPrintIdsOnly
	oldInteresting, oldUninteresting, oldOutdated := searchInteresting, searchUninteresting, searchOutdated
	oldLabels, oldFilters, oldInverse := searchLabelRegexps, searchCustomFilterNames, searchInverseCustomFilterNames
	oldPlugins, oldHuman := machineSearchPlugins, prnt.NoHumanOnly
	oldPageSize, oldPageToken := searchPageSize, searchPageToken
	defer func() {
		searchPrintJson, searchPrintIdsOnly = oldJSON, oldIDs
		searchInteresting, searchUninteresting, searchOutdated = oldInteresting, oldUninteresting, oldOutdated
		searchLabelRegexps, searchCustomFilterNames, searchInverseCustomFilterNames = oldLabels, oldFilters, oldInverse
		machineSearchPlugins, prnt.NoHumanOnly = oldPlugins, oldHuman
		searchPageSize, searchPageToken = oldPageSize, oldPageToken
	}()
	searchPrintJson, prnt.NoHumanOnly = true, true
	machineSearchPlugins = func() []*plugin.Plugin {
		return []*plugin.Plugin{{
			Name: "test",
			MessageInterest: func(m *gm.Message, _ *api.MsgHelper) plugin.InterestLevel {
				if m.Id == "a" {
					return plugin.StronglyInteresting
				}
				return plugin.StronglyUninteresting
			},
			DetailRequiredForInterest: func() api.MessageDetailLevel { return api.LabelsOnly },
			OutdatedMessages: func(q string, _ *api.MsgHelper, max int64) []*gm.Message {
				if q != "query" || max != searchMaxMsgs {
					t.Errorf("outdated query=%q max=%d", q, max)
				}
				return []*gm.Message{{Id: "b"}, {Id: "outside-query"}}
			},
			MessageFilters: map[string]*plugin.MessageFilter{
				"a-only": {Matches: func(m *gm.Message, h *api.MsgHelper) bool {
					// Plugins can fetch details themselves, as in ordinary search.
					loaded, err := h.GetMessage(m.Id, api.LabelsOnly)
					if err != nil {
						t.Error(err)
						return false
					}
					return loaded.Snippet == "a"
				}},
				"none": {Matches: func(*gm.Message, *api.MsgHelper) bool { return false }},
			},
		}}
	}
	listCalls := 0
	fakeMachine(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("unexpected mutation: %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/gmail/v1/users/me/profile":
			fmt.Fprint(w, `{"emailAddress":"a@b"}`)
		case "/gmail/v1/users/me/labels":
			fmt.Fprint(w, `{"labels":[{"id":"L","name":"Work"}]}`)
		case "/gmail/v1/users/me/messages":
			listCalls++
			if r.URL.Query().Get("q") != "query" {
				t.Errorf("query: %s", r.URL)
			}
			if r.URL.Query().Get("pageToken") == "" {
				fmt.Fprint(w, `{"messages":[{"id":"a","threadId":"ta"}],"nextPageToken":"next"}`)
			} else {
				fmt.Fprint(w, `{"messages":[{"id":"b","threadId":"tb"}]}`)
			}
		default:
			id := strings.TrimPrefix(r.URL.Path, "/gmail/v1/users/me/messages/")
			if id != "a" && id != "b" {
				t.Errorf("unexpected request: %s", r.URL)
			}
			labels := `[]`
			if id == "a" {
				labels = `["L"]`
			}
			fmt.Fprintf(w, `{"id":%q,"threadId":%q,"snippet":%q,"labelIds":%s,"payload":{"headers":[{"name":"Subject","value":"test"}]}}`, id, "t"+id, id, labels)
		}
	})
	cases := []struct {
		name                                 string
		labels, filters, inverse             []string
		interesting, uninteresting, outdated bool
		want                                 string
	}{
		{name: "xfilter", filters: []string{"a-only"}, want: "a"},
		{name: "not-xfilter", inverse: []string{"a-only"}, want: "b"},
		{name: "combined", filters: []string{"a-only"}, inverse: []string{"none"}, want: "a"},
		{name: "labelp", labels: []string{"^Work$"}, want: "a"},
		{name: "interesting", interesting: true, want: "a"},
		{name: "uninteresting", uninteresting: true, want: "b"},
		{name: "outdated", outdated: true, want: "b"},
		{name: "empty", filters: []string{"none"}},
	}
	for _, tc := range cases {
		for _, mode := range []struct {
			name                       string
			idsOnly, paging, tokenOnly bool
		}{
			{name: "ordinary"}, {name: "ids-only", idsOnly: true},
			{name: "paged", paging: true}, {name: "paged-ids-only", paging: true, idsOnly: true},
			{name: "page-token-only", paging: true, tokenOnly: true},
		} {
			t.Run(tc.name+"/"+mode.name, func(t *testing.T) {
				searchPrintIdsOnly = mode.idsOnly
				searchPageToken, searchPageSize = "", 500
				searchLabelRegexps, searchCustomFilterNames, searchInverseCustomFilterNames = tc.labels, tc.filters, tc.inverse
				searchInteresting, searchUninteresting, searchOutdated = tc.interesting, tc.uninteresting, tc.outdated
				c := &cobra.Command{}
				var b bytes.Buffer
				c.SetOut(&b)
				var result []map[string]interface{}
				if mode.paging {
					c.Flags().StringVar(&searchPageToken, "page-token", "", "")
					if mode.tokenOnly {
						if err := c.Flags().Set("page-token", ""); err != nil {
							t.Fatal(err)
						}
					} else {
						c.Flags().Int64Var(&searchPageSize, "page-size", 500, "")
						if err := c.Flags().Set("page-size", "1"); err != nil {
							t.Fatal(err)
						}
					}
					for page := 0; page < 2; page++ {
						b.Reset()
						before := listCalls
						if err := runSearchCmd(c, []string{"query"}); err != nil {
							t.Fatal(err)
						}
						if listCalls != before+1 {
							t.Fatalf("paged request fetched %d Gmail pages", listCalls-before)
						}
						var envelope map[string]json.RawMessage
						if err := json.Unmarshal(b.Bytes(), &envelope); err != nil {
							t.Fatal(err)
						}
						var messages []map[string]interface{}
						if err := json.Unmarshal(envelope["messages"], &messages); err != nil {
							t.Fatal(err)
						}
						if string(envelope["messages"]) == "null" {
							t.Fatalf("expected array: %s", b.String())
						}
						result = append(result, messages...)
						if page == 0 {
							if string(envelope["nextPageToken"]) != `"next"` {
								t.Fatalf("lost continuation token: %s", b.String())
							}
							if tc.want != "a" && len(messages) != 0 {
								t.Fatalf("expected empty intermediate page: %s", b.String())
							}
							if err := c.Flags().Set("page-token", "next"); err != nil {
								t.Fatal(err)
							}
						} else if _, exists := envelope["nextPageToken"]; exists {
							t.Fatalf("final page retained token: %s", b.String())
						}
					}
				} else {
					if err := runSearchCmd(c, []string{"query"}); err != nil {
						t.Fatal(err)
					}
					if err := json.Unmarshal(b.Bytes(), &result); err != nil {
						t.Fatal(err, b.String())
					}
				}
				if tc.want == "" {
					if len(result) != 0 {
						t.Fatalf("expected empty results, got %v", result)
					}
				} else {
					key := "messageId"
					if mode.idsOnly || mode.paging {
						key = "id"
						if len(result) != 1 || len(result[0]) != 2 {
							t.Fatalf("message details leaked: %v", result)
						}
					}
					if len(result) != 1 || result[0][key] != tc.want {
						t.Fatalf("unexpected result: %v", result)
					}
				}
			})
		}
	}
}

func TestJSONSearchFilterValidation(t *testing.T) {
	oldJSON, oldInteresting, oldUninteresting := searchPrintJson, searchInteresting, searchUninteresting
	oldFilters, oldLabels := searchCustomFilterNames, searchLabelRegexps
	defer func() {
		searchPrintJson, searchInteresting, searchUninteresting = oldJSON, oldInteresting, oldUninteresting
		searchCustomFilterNames, searchLabelRegexps = oldFilters, oldLabels
	}()
	searchPrintJson = true
	// Validation must fail before authentication or message access.
	fakeMachine(t, func(http.ResponseWriter, *http.Request) { t.Fatal("unexpected Gmail request") })
	c := &cobra.Command{}
	searchInteresting, searchUninteresting = true, true
	if err := runSearchCmd(c, nil); err == nil || !strings.Contains(err.Error(), "mutually exclusive") {
		t.Fatalf("got %v", err)
	}
	searchInteresting, searchUninteresting = false, false
	searchLabelRegexps = []string{"["}
	if err := runSearchCmd(c, nil); err == nil || !strings.Contains(err.Error(), "invalid --labelp") {
		t.Fatalf("got %v", err)
	}
	searchLabelRegexps = nil
	c.Flags().Int64("page-size", 500, "")
	if err := c.Flags().Set("page-size", "500"); err != nil {
		t.Fatal(err)
	}
	c.Flags().Int64("max", -1, "")
	if err := c.Flags().Set("max", "5"); err != nil {
		t.Fatal(err)
	}
	if err := runSearchCmd(c, nil); err == nil || !strings.Contains(err.Error(), "paging cannot be combined with --max") {
		t.Fatalf("got %v", err)
	}
}

func TestCustomSearchFilterUnknownName(t *testing.T) {
	oldFilters, oldInverse := searchCustomFilterNames, searchInverseCustomFilterNames
	defer func() { searchCustomFilterNames, searchInverseCustomFilterNames = oldFilters, oldInverse }()
	h := &GmailHelper{plugins: []*plugin.Plugin{}}
	for _, inverse := range []bool{false, true} {
		searchCustomFilterNames, searchInverseCustomFilterNames = nil, nil
		if inverse {
			searchInverseCustomFilterNames = []string{"missing"}
		} else {
			searchCustomFilterNames = []string{"missing"}
		}
		if _, err := applyCustomFilters(nil, h); err == nil || !strings.Contains(err.Error(), "'missing' is not an available xfilter") {
			t.Fatalf("got %v", err)
		}
	}
}
