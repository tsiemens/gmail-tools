package cmd

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
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
