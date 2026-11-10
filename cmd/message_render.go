package cmd

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/tsiemens/gmail-tools/prnt"
	"github.com/tsiemens/gmail-tools/util"
	gm "google.golang.org/api/gmail/v1"
)

// Only the matched messages participate in grouping and shared labels.
// Rendering never fetches additional messages from a thread.
type messageRenderer struct {
	labels      func(*gm.Message) []string
	labelColors map[string]string
}

func displayText(s string) string {
	// Headers and user labels must not inject terminal controls or extra rows.
	return strings.Map(func(r rune) rune {
		if r < 32 || (r >= 127 && r <= 159) {
			return ' '
		}
		return r
	}, s)
}

func messageHeader(m *gm.Message, name, fallback string) string {
	if m.Payload != nil {
		for _, h := range m.Payload.Headers {
			if strings.EqualFold(h.Name, name) && strings.TrimSpace(h.Value) != "" {
				return displayText(h.Value)
			}
		}
	}
	return fallback
}

func (r messageRenderer) visibleLabels(m *gm.Message) []string {
	var labels []string
	seen := map[string]bool{}
	for _, l := range r.labels(m) {
		if !util.DebugMode && (l == "INBOX" || strings.HasPrefix(l, "CATEGORY_")) {
			continue
		}
		if !seen[l] {
			labels = append(labels, l)
			seen[l] = true
		}
	}
	sort.Strings(labels)
	return labels
}

func (r messageRenderer) formatLabels(labels []string) string {
	if len(labels) == 0 {
		return ""
	}
	styled := make([]string, 0, len(labels))
	for _, l := range labels {
		style := prnt.Style().FgYellow()
		if color := r.labelColors[l]; color != "" {
			// Use the same configured color names as the existing renderer.
			if code, ok := util.Colors[color]; ok {
				styled = append(styled, prnt.Style().ANSI(code).Bold().On(displayText(l)))
				continue
			}
		}
		styled = append(styled, style.On(displayText(l)))
	}
	return "  [" + strings.Join(styled, ", ") + "]"
}

func (r messageRenderer) thread(w io.Writer, input []*gm.Message, indent string, mainCategory ...string) {
	msgs := append([]*gm.Message(nil), input...)
	sort.Slice(msgs, func(i, j int) bool {
		if msgs[i].InternalDate != msgs[j].InternalDate {
			return msgs[i].InternalDate < msgs[j].InternalDate
		}
		return msgs[i].Id < msgs[j].Id
	})
	labels := make([][]string, len(msgs))
	counts := map[string]int{}
	for i, m := range msgs {
		labels[i] = r.visibleLabels(m)
		for _, l := range labels[i] {
			counts[l]++
		}
	}
	var shared []string
	for _, l := range labels[0] {
		if counts[l] == len(msgs) {
			shared = append(shared, l)
		}
	}
	subject := "<No subject>"
	for _, m := range msgs {
		if s := messageHeader(m, "Subject", ""); s != "" {
			subject = s
			break
		}
	}
	count := fmt.Sprintf("%d matched messages", len(msgs))
	if len(msgs) == 1 {
		count = "1 matched message"
	}
	fmt.Fprintf(w, "%s┌ %s%s  %s  %s\n", indent, prnt.Style().Bold().On(subject), r.formatLabels(shared), prnt.Style().Dim().On("("+count+")"), prnt.Style().Dim().On("thread:"+displayText(msgs[0].ThreadId)))
	for i, m := range msgs {
		branch := "├"
		if i == len(msgs)-1 {
			branch = "└"
		}
		date := "<unknown date>"
		if m.InternalDate > 0 {
			date = time.UnixMilli(m.InternalDate).Local().Format("2006-01-02 15:04 MST")
		}
		var own []string
		for _, l := range labels[i] {
			if counts[l] != len(msgs) {
				own = append(own, l)
			}
		}
		if len(mainCategory) > 0 {
			cats := messageCategories(m)
			for _, c := range cats {
				if c != mainCategory[0] {
					own = append(own, c)
				}
			}
		}
		fmt.Fprintf(w, "%s%s %s  %s%s  %s\n", indent, branch, prnt.Style().Dim().On(date), prnt.Style().FgCyan().On(messageHeader(m, "From", "<unknown sender>")), r.formatLabels(own), prnt.Style().Dim().On("msg:"+displayText(m.Id)))
	}
}

var searchCategories = []string{"PERSONAL", "SOCIAL", "PROMOTIONS", "UPDATES", "FORUMS", "NO CATEGORY"}

// Uncategorized sent messages neither vote for NO CATEGORY nor show that tag.
func messageCategories(m *gm.Message) []string {
	labels := map[string]bool{}
	for _, l := range m.LabelIds {
		labels[l] = true
	}
	var categories []string
	for _, c := range searchCategories[:5] {
		if labels["CATEGORY_"+c] {
			categories = append(categories, c)
		}
	}
	if len(categories) == 0 && !labels["SENT"] {
		categories = append(categories, "NO CATEGORY")
	}
	return categories
}

func threadCategory(msgs []*gm.Message) string {
	counts := map[string]int{}
	for _, m := range msgs {
		for _, c := range messageCategories(m) {
			counts[c]++
		}
	}
	best, max := "NO CATEGORY", 0
	// Fixed display order breaks ties, preferring a real category over none.
	for _, c := range searchCategories {
		if counts[c] > max {
			best, max = c, counts[c]
		}
	}
	return best
}

func (r messageRenderer) categories(w io.Writer, msgs []*gm.Message) {
	allThreads := map[string][]*gm.Message{}
	for _, m := range msgs {
		key := m.ThreadId
		if key == "" {
			key = "message:" + m.Id
		}
		allThreads[key] = append(allThreads[key], m)
	}
	byCategory := map[string]map[string][]*gm.Message{}
	for key, thread := range allThreads {
		category := threadCategory(thread)
		if byCategory[category] == nil {
			byCategory[category] = map[string][]*gm.Message{}
		}
		byCategory[category][key] = thread
	}
	for _, category := range searchCategories {
		threads := byCategory[category]
		if len(threads) == 0 {
			continue
		}
		fmt.Fprintln(w, prnt.Style().Bold().FgMagenta().On(category))
		keys := make([]string, 0, len(threads))
		newest := map[string]int64{}
		for k, ms := range threads {
			keys = append(keys, k)
			for _, m := range ms {
				if m.InternalDate > newest[k] {
					newest[k] = m.InternalDate
				}
			}
		}
		sort.Slice(keys, func(i, j int) bool {
			if newest[keys[i]] != newest[keys[j]] {
				return newest[keys[i]] > newest[keys[j]]
			}
			return keys[i] < keys[j]
		})
		for _, k := range keys {
			r.thread(w, threads[k], "", category)
			fmt.Fprintln(w)
		}
	}
}
