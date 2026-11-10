package cmd

import (
	"bytes"
	"strings"
	"testing"

	gm "google.golang.org/api/gmail/v1"
)

func TestThreadRendering(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	r := messageRenderer{labels: func(m *gm.Message) []string { return m.LabelIds }}
	old := &gm.Message{Id: "old", ThreadId: "thread", InternalDate: 1700000000000, LabelIds: []string{"Work", "INBOX"}, Payload: &gm.MessagePart{Headers: []*gm.MessagePartHeader{{Name: "Subject", Value: "Project"}, {Name: "From", Value: "Alice"}}}}
	recent := &gm.Message{Id: "new", ThreadId: "thread", InternalDate: 1700003600000, LabelIds: []string{"Work", "UNREAD"}, Payload: &gm.MessagePart{Headers: []*gm.MessagePartHeader{{Name: "Subject", Value: "Re: Project"}, {Name: "From", Value: "Bob"}}}}
	var out bytes.Buffer
	r.thread(&out, []*gm.Message{recent, old}, "")
	s := out.String()
	for _, want := range []string{"┌ Project", "├ ", "└ ", "thread:thread", "msg:old", "msg:new", "2023-11-", "[UNREAD]", "2 matched messages"} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q in %s", want, s)
		}
	}
	if strings.Count(s, "Work") != 1 || strings.Contains(s, "Re: Project") || strings.Contains(s, "INBOX") || strings.Contains(s, "\033") {
		t.Fatal(s)
	}
	if strings.Index(s, "Alice") > strings.Index(s, "Bob") {
		t.Fatal("messages not chronological")
	}
	if recent.Id != "new" {
		t.Fatal("input mutated")
	}
}

func TestCategoryOrderingAndMissingMetadata(t *testing.T) {
	r := messageRenderer{labels: func(m *gm.Message) []string { return m.LabelIds }}
	var out bytes.Buffer
	r.categories(&out, []*gm.Message{{Id: "old", ThreadId: "a", InternalDate: 1}, {Id: "new", ThreadId: "b", InternalDate: 2}, {Id: "missing"}})
	s := out.String()
	if strings.Index(s, "msg:new") > strings.Index(s, "msg:old") {
		t.Fatal(s)
	}
	for _, want := range []string{"<No subject>", "<unknown sender>", "<unknown date>", "msg:missing"} {
		if !strings.Contains(s, want) {
			t.Fatal(s)
		}
	}
}

func TestDisplayText(t *testing.T) {
	if got := displayText("subject\n\033[31m\t"); got != "subject  [31m " {
		t.Fatalf("unsafe display text: %q", got)
	}
}

func TestThreadsStayTogetherAcrossCategories(t *testing.T) {
	r := messageRenderer{labels: func(m *gm.Message) []string { return m.LabelIds }}
	msgs := []*gm.Message{
		{Id: "received1", ThreadId: "mixed", LabelIds: []string{"CATEGORY_UPDATES"}},
		{Id: "received2", ThreadId: "mixed", LabelIds: []string{"CATEGORY_UPDATES"}},
		{Id: "other", ThreadId: "mixed", LabelIds: []string{"CATEGORY_SOCIAL"}},
		{Id: "sent", ThreadId: "mixed", LabelIds: []string{"SENT"}},
		{Id: "uncategorized", ThreadId: "mixed"},
	}
	var out bytes.Buffer
	r.categories(&out, msgs)
	s := out.String()
	if !strings.HasPrefix(s, "UPDATES\n") || strings.Count(s, "thread:mixed") != 1 {
		t.Fatal(s)
	}
	for _, m := range msgs {
		if strings.Count(s, "msg:"+m.Id) != 1 {
			t.Fatal(s)
		}
	}
	for _, line := range strings.Split(s, "\n") {
		if strings.Contains(line, "msg:other") && !strings.Contains(line, "[SOCIAL]") {
			t.Fatal(line)
		}
		if strings.Contains(line, "msg:uncategorized") && !strings.Contains(line, "[NO CATEGORY]") {
			t.Fatal(line)
		}
		if strings.Contains(line, "msg:sent") && strings.Contains(line, "NO CATEGORY") {
			t.Fatal(line)
		}
	}
}

func TestThreadCategoryVotes(t *testing.T) {
	tests := []struct {
		name   string
		labels [][]string
		want   string
	}{
		{"majority", [][]string{{"CATEGORY_UPDATES"}, {"CATEGORY_SOCIAL"}, {"CATEGORY_UPDATES"}}, "UPDATES"},
		{"tie", [][]string{{"CATEGORY_UPDATES"}, {"CATEGORY_SOCIAL"}}, "SOCIAL"},
		{"sent do not vote", [][]string{{"SENT"}, {"SENT"}, {"CATEGORY_UPDATES"}}, "UPDATES"},
		{"only sent", [][]string{{"SENT"}}, "NO CATEGORY"},
		{"uncategorized majority", [][]string{nil, nil, {"CATEGORY_UPDATES"}}, "NO CATEGORY"},
		{"multiple labels", [][]string{{"CATEGORY_UPDATES", "CATEGORY_SOCIAL"}}, "SOCIAL"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var msgs []*gm.Message
			for _, labels := range tt.labels {
				msgs = append(msgs, &gm.Message{LabelIds: labels})
			}
			if got := threadCategory(msgs); got != tt.want {
				t.Fatalf("got %s, want %s", got, tt.want)
			}
		})
	}
}
