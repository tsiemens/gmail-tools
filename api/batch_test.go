package api

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strings"
	"testing"

	gm "google.golang.org/api/gmail/v1"
)

func TestApplyLabelsMessageChecks(t *testing.T) {
	for _, mode := range []string{"batch", "missing", "truncated", "skip"} {
		t.Run(mode, func(t *testing.T) {
			batches, mutations := 0, 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case strings.HasSuffix(r.URL.Path, "/labels"):
					fmt.Fprint(w, `{"labels":[{"id":"label1","name":"test"}]}`)
				case r.URL.Path == "/batch/gmail/v1":
					batches++
					_, params, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
					requestBody, _ := io.ReadAll(r.Body)
					reader := multipart.NewReader(bytes.NewReader(requestBody), params["boundary"])
					writer := multipart.NewWriter(w)
					w.Header().Set("Content-Type", "multipart/mixed; boundary="+writer.Boundary())
					count := 0
					for {
						part, err := reader.NextPart()
						if err == io.EOF {
							break
						}
						if err != nil {
							t.Error(err)
							break
						}
						req, err := http.ReadRequest(bufio.NewReader(part))
						if err != nil {
							t.Error(err)
							break
						}
						if req.URL.Query().Get("format") != "minimal" {
							t.Error("missing minimal format")
						}
						count++
						if mode == "truncated" {
							continue
						}
						out, _ := writer.CreatePart(textproto.MIMEHeader{"Content-Type": {"application/http"}})
						status := "200 OK"
						if mode == "missing" && count == 2 {
							status = "404 Not Found"
						}
						fmt.Fprintf(out, "HTTP/1.1 %s\r\nContent-Type: application/json\r\n\r\n{}", status)
					}
					if count > 50 {
						t.Errorf("batch too large: %d", count)
					}
					writer.Close()
				case strings.HasSuffix(r.URL.Path, "/messages/batchModify"):
					mutations++
					w.WriteHeader(204)
				default:
					t.Errorf("unexpected request %s", r.URL)
					w.WriteHeader(500)
				}
			}))
			defer server.Close()
			srv, _ := gm.New(server.Client())
			srv.BasePath = server.URL + "/gmail/v1/"
			helper := NewMsgHelper("me", srv, false, server.Client())
			helper.SkipMessageChecks = mode == "skip"
			ids := make([]string, 51)
			for i := range ids {
				ids[i] = fmt.Sprint(i)
			}
			err := helper.ApplyLabelsByIdIter(SizedMessageIdIteratorFromIds(ids), LabelsFromLabelNames([]string{"test"}), nil)
			failed := mode == "missing" || mode == "truncated"
			if (err != nil) != failed {
				t.Fatalf("error = %v", err)
			}
			if failed && mutations != 0 {
				t.Fatal("mutated after failed check")
			}
			if !failed && mutations != 1 {
				t.Fatalf("mutations = %d", mutations)
			}
			if mode == "skip" && batches != 0 {
				t.Fatal("skip performed checks")
			}
			if mode == "batch" && batches != 2 {
				t.Fatalf("batches = %d", batches)
			}
		})
	}
}
