package api

import (
	"bytes"
	"encoding/base64"
	"regexp"
	"strings"

	gm "google.golang.org/api/gmail/v1"
)

var fromFieldRegexp = regexp.MustCompile(`\s*(\S|\S.*\S)\s*<.*>\s*`)

func GetFromName(fromHeaderVal string) string {
	matches := fromFieldRegexp.FindStringSubmatch(fromHeaderVal)
	if len(matches) > 0 {
		return matches[1]
	}
	return fromHeaderVal
}

func decodePartBody(part *gm.MessagePart) string {
	data := part.Body.Data
	decoder := base64.NewDecoder(base64.URLEncoding, strings.NewReader(data))
	buf := new(bytes.Buffer)
	buf.ReadFrom(decoder)
	b := buf.Bytes()
	return string(b[:])
}

// Decodes the messages' body text, putting each part as a separate entry in the
// returned slice. Will be at least size 1
func GetMessageBody(msg *gm.Message) []string {
	partTexts := make([]string, 0, 1+len(msg.Payload.Parts))
	partTexts = append(partTexts, decodePartBody(msg.Payload))
	for _, part := range msg.Payload.Parts {
		// For multipart messages
		partTexts = append(partTexts, decodePartBody(part))
	}
	return partTexts
}

func MessageHasBody(msg *gm.Message) bool {
	return msg.Payload != nil && msg.Payload.Body != nil
}
