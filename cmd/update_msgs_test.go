package cmd

import (
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"testing"

	gm "google.golang.org/api/gmail/v1"
)

func TestMessageIDsForThreads(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(fmt.Sprint(fail), func(t *testing.T) {
			var calls []string
			srv, err := gm.New(&http.Client{Transport: handlerTransport{func(w http.ResponseWriter, r *http.Request) {
				calls = append(calls, r.URL.Path)
				if r.Method != "GET" || !strings.Contains(r.URL.Path, "/threads/") {
					t.Errorf("unexpected request %s %s", r.Method, r.URL)
				}
				if r.URL.Query().Get("format") != "minimal" || r.URL.Query().Get("fields") != "messages/id" {
					t.Errorf("unexpected query %s", r.URL.RawQuery)
				}
				w.Header().Set("Content-Type", "application/json")
				if strings.HasSuffix(r.URL.Path, "/a") {
					fmt.Fprint(w, `{"messages":[{"id":"1"},{"id":"2"}]}`)
					return
				}
				if fail {
					w.WriteHeader(404)
					fmt.Fprint(w, `{"error":{"code":404,"message":"missing"}}`)
					return
				}
				fmt.Fprint(w, `{"messages":[{"id":"2"},{"id":"3"}]}`)
			}}})
			if err != nil {
				t.Fatal(err)
			}
			ids, err := messageIDsForThreads(srv, []string{"a", "a", "b"})
			if len(calls) != 2 {
				t.Fatalf("calls: %v", calls)
			}
			if fail {
				if err == nil || !strings.Contains(err.Error(), "thread b") || ids != nil {
					t.Fatalf("ids %v, error %v", ids, err)
				}
			} else if err != nil || !reflect.DeepEqual(ids, []string{"1", "2", "3"}) {
				t.Fatalf("ids %v, error %v", ids, err)
			}
		})
	}
}
