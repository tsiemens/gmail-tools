package template

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	f "github.com/tsiemens/gmail-tools/filter"
)

const (
	META_LABEL         = "M3TA"
	META_PRIMARY_LABEL = "M3TAP"
)

var metaRegexp *regexp.Regexp = regexp.MustCompile("^M3TAP?$")

type MetaKey struct {
	labels []string
}

func NewMetaKey(labels []string) MetaKey {
	lCopy := make([]string, len(labels))
	copy(lCopy, labels)
	return MetaKey{lCopy}
}

func (k MetaKey) Equals(other MetaKey) bool {
	if len(k.labels) != len(other.labels) {
		return false
	}
	for i, l := range other.labels {
		if k.labels[i] != l {
			return false
		}
	}
	return true
}

// If elem is of the format (M3TA x y z) or "M3TA x y z"
// returns a sorted slice {"x", "y", "z"}
func findMetaGroupKey(elem *f.FilterElement, primaryOnly bool) (MetaKey, bool, error) {
	ok := false
	var key MetaKey

	var tagsAndMeta []string
	if elem.Delims == "()" && elem.HasSubElems() {
		for _, se := range elem.SubElems {
			tagsAndMeta = append(tagsAndMeta, strings.TrimSpace(se.FullFilterStr()))
		}
	} else if elem.Delims == "\"\"" {
		tagsAndMeta = strings.Split(elem.FilterStr, " ")
	} else {
		// Not a group that can hold a M3TA tag
		return key, ok, nil
	}

	var tags []string
	for _, t := range tagsAndMeta {
		isMeta := (primaryOnly && META_PRIMARY_LABEL == t) ||
			(!primaryOnly && metaRegexp.MatchString(t))
		if !isMeta {
			tags = append(tags, t)
		}
	}

	if len(tagsAndMeta) == len(tags) {
		// No meta tag in the group
		return key, ok, nil
	}
	if len(tags) == 0 {
		// No labels in the meta group
		return key, ok, fmt.Errorf("Template meta group has no labels: %s",
			elem.FullFilterStr())
	}
	sort.Strings(tags)
	ok = true
	key = NewMetaKey(tags)
	return key, ok, nil
}

func FindMetaGroupKey(elem *f.FilterElement) (MetaKey, bool, error) {
	return findMetaGroupKey(elem, false)
}

func FindPrimaryMetaGroupKey(elem *f.FilterElement) (MetaKey, bool, error) {
	return findMetaGroupKey(elem, true)
}
