package api

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"regexp"
	"sort"
	"strings"

	gm "google.golang.org/api/gmail/v1"
)

var SpecialLabelNames = []string{
	"UNREAD",
	"CATEGORY_PERSONAL",
	"CATEGORY_SOCIAL",
	"CATEGORY_PROMOTIONS",
	"CATEGORY_UPDATES",
	"CATEGORY_FORUMS",
	"IMPORTANT",
	"INBOX",
}

// Determined heuristically from testing
// We should avoid making more than this many rquests in parallel, else
// the gmail service will yield errors.
// The API does not publicly say what the limit is at the moment.
var MaxConcurrentRequests = 5

// A wrapper for a message Id that can be passed by reference.
// This helps save a little space, and reduces confusion about what contents
// a message contains when returned from certain functions.
type MessageId struct {
	Id string
}

type EmailAddress struct {
	Address string
	Name    string
}

type Headers struct {
	Subject string
	From    EmailAddress
	To      []EmailAddress
	Cc      []EmailAddress
}

var namedEmailRegexp = regexp.MustCompile(`\s*(\S|\S.*\S)\s*<(.*)>\s*`)
var unnamedEmailRegexp = regexp.MustCompile(`^\s*(\S*)\s*$`)

func GetEmailsInField(fromHeaderVal string) []EmailAddress {
	emails := make([]EmailAddress, 0)
	emailStrs := strings.Split(fromHeaderVal, ",")
	for _, emailStr := range emailStrs {
		matches := namedEmailRegexp.FindStringSubmatch(emailStr)
		if len(matches) > 1 {
			emails = append(emails, EmailAddress{
				Address: matches[2],
				Name:    matches[1]})
		} else {
			// Try unnamed
			matches = unnamedEmailRegexp.FindStringSubmatch(emailStr)
			if len(matches) > 0 {
				emails = append(emails, EmailAddress{
					Address: matches[1],
					Name:    ""})
			}
		}
	}
	return emails
}

func GetMsgHeaders(msg *gm.Message) (*Headers, error) {
	if msg.Payload != nil && msg.Payload.Headers != nil {
		headers := &Headers{}
		for _, hdr := range msg.Payload.Headers {
			if hdr.Name == "Subject" {
				headers.Subject = hdr.Value
			} else if hdr.Name == "From" {
				emails := GetEmailsInField(hdr.Value)
				if len(emails) > 0 {
					headers.From = emails[0]
				}
			} else if hdr.Name == "To" {
				headers.To = GetEmailsInField(hdr.Value)
			} else if hdr.Name == "Cc" {
				headers.Cc = GetEmailsInField(hdr.Value)
			}
		}
		return headers, nil
	}
	return nil, fmt.Errorf("No headers found")
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

func MessagesLatestFirst(msgs []*gm.Message) []*gm.Message {
	sorted := make([]*gm.Message, len(msgs))
	copy(sorted, msgs)

	// func Slice(slice interface{}, less func(i, j int) bool)
	msgLess := func(i, j int) bool {
		// Backwards, since we want the higher dates first.
		return sorted[i].InternalDate > sorted[j].InternalDate
	}

	sort.Slice(sorted, msgLess)
	return sorted
}

func MessageIdsByThread(msgs []*gm.Message) map[string][]*MessageId {
	threads := make(map[string][]*MessageId)
	for _, msg := range msgs {
		if tMsgs, ok := threads[msg.ThreadId]; ok {
			threads[msg.ThreadId] = append(tMsgs, &MessageId{msg.Id})
		} else {
			threads[msg.ThreadId] = []*MessageId{&MessageId{msg.Id}}
		}
	}
	return threads
}

func MessagesByThread(msgs []*gm.Message) map[string][]*gm.Message {
	threads := make(map[string][]*gm.Message)
	for _, msg := range msgs {
		if tMsgs, ok := threads[msg.ThreadId]; ok {
			threads[msg.ThreadId] = append(tMsgs, msg)
		} else {
			threads[msg.ThreadId] = []*gm.Message{msg}
		}
	}
	return threads
}
