package api

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"

	"google.golang.org/api/googleapi"
)

const messageCheckBatchSize = 50

// Check every message before allowing any mutation, including across batch boundaries.
func (h *MsgHelper) checkMessagesBatched(ids []string) error {
	for start := 0; start < len(ids); start += messageCheckBatchSize {
		end := start + messageCheckBatchSize
		if end > len(ids) {
			end = len(ids)
		}
		if err := h.checkMessageBatch(ids[start:end]); err != nil {
			return err
		}
	}
	return nil
}

func (h *MsgHelper) checkMessageBatch(ids []string) error {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for _, id := range ids {
		part, err := writer.CreatePart(textproto.MIMEHeader{"Content-Type": {"application/http"}})
		if err != nil {
			return err
		}
		path := "/gmail/v1/users/" + url.PathEscape(h.User) + "/messages/" + url.PathEscape(id)
		fmt.Fprintf(part, "GET %s?format=minimal&fields=id HTTP/1.1\r\n\r\n", path)
	}
	if err := writer.Close(); err != nil {
		return err
	}
	endpoint, err := url.Parse(h.srv.BasePath)
	if err != nil {
		return err
	}
	endpoint.Path, endpoint.RawPath, endpoint.RawQuery = "/batch/gmail/v1", "", ""
	req, err := http.NewRequest(http.MethodPost, endpoint.String(), &body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "multipart/mixed; boundary="+writer.Boundary())
	resp, err := h.batchClient.Do(req)
	if err != nil {
		return fmt.Errorf("message checks: %w", err)
	}
	defer resp.Body.Close()
	if err := googleapi.CheckResponse(resp); err != nil {
		return err
	}
	mediaType, params, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if err != nil || mediaType != "multipart/mixed" || params["boundary"] == "" {
		return fmt.Errorf("invalid message check batch response content type")
	}
	reader := multipart.NewReader(resp.Body, params["boundary"])
	for _, id := range ids {
		part, err := reader.NextPart()
		if err != nil {
			return fmt.Errorf("message %s: incomplete batch response: %w", id, err)
		}
		inner, err := http.ReadResponse(bufio.NewReader(part), nil)
		if err != nil {
			part.Close()
			return fmt.Errorf("message %s: %w", id, err)
		}
		err = googleapi.CheckResponse(inner)
		inner.Body.Close()
		part.Close()
		if err != nil {
			return fmt.Errorf("message %s: %w", id, err)
		}
	}
	if _, err := reader.NextPart(); err != io.EOF {
		return fmt.Errorf("unexpected extra batch response: %v", err)
	}
	return nil
}
