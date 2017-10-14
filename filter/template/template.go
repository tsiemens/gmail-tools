package template

import (
	"fmt"
	"hash/fnv"
	"regexp"
	"sort"
	"strings"

	hashmap "github.com/tsiemens/go-concurrentMap"

	f "github.com/tsiemens/gmail-tools/filter"
	"github.com/tsiemens/gmail-tools/util"
)

const (
	META_LABEL         = "M3TA"
	META_PRIMARY_LABEL = "M3TAP"
)

var metaRegexp *regexp.Regexp = regexp.MustCompile("^M3TAP?$")

func intToBytes(intVal interface{}) []byte {
	var length uint8
	var uval uint64

	switch tp := intVal.(type) {
	case int32:
		length = 4
		uval = uint64(intVal.(int32))
	case uint32:
		length = 4
		uval = uint64(intVal.(uint32))
	case int64:
		length = 8
		uval = uint64(intVal.(int64))
	case uint64:
		length = 8
		uval = intVal.(uint64)
	default:
		util.Assertf(false, "Unsupported type: %+v, val:%+v", tp, intVal)
	}

	bts := make([]byte, length)
	for i := uint8(0); i < length; i++ {
		bts[i] = byte((uval >> (8 * i)) & 0xFF)
	}
	return bts
}

type MetaKey struct {
	labels []string
}

var _ hashmap.Hashable = MetaKey{}

func NewMetaKey(labels []string) MetaKey {
	lCopy := make([]string, len(labels))
	copy(lCopy, labels)
	return MetaKey{lCopy}
}

func (k MetaKey) String() string {
	return fmt.Sprintf("%v", k.labels)
}

func (k MetaKey) Equals(otherI interface{}) bool {
	other, ok := otherI.(MetaKey)
	if !ok {
		return false
	}
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

func (k MetaKey) Hash() uint64 {
	h := fnv.New64()
	for i, l := range k.labels {
		h.Write(intToBytes(uint32(i)))
		h.Write([]byte(l))
	}
	return h.Sum64()
}

func (k MetaKey) HashBytes() []byte {
	return intToBytes(k.Hash())
}

type MetaKeySet struct {
	hmap *hashmap.ConcurrentMap
}

func NewMetaKeySet(keys ...MetaKey) *MetaKeySet {
	set := &MetaKeySet{hashmap.NewConcurrentMap(10)}
	for _, k := range keys {
		set.Add(k)
	}
	return set
}

func (s *MetaKeySet) Len() int32 {
	return s.hmap.Size()
}

func (s *MetaKeySet) Add(k MetaKey) {
	_, err := s.hmap.Put(k, byte(1))
	util.Assert(err == nil, err)
}

func (s *MetaKeySet) Contains(k MetaKey) bool {
	found, err := s.hmap.ContainsKey(k)
	util.Assert(err == nil, err)
	return found
}

func (s *MetaKeySet) Keys() []MetaKey {
	var keys []MetaKey
	for _, entry := range s.hmap.ToSlice() {
		keys = append(keys, entry.Key().(MetaKey))
	}
	return keys
}

func (s *MetaKeySet) UpdateSet(other *MetaKeySet) {
	for _, entry := range other.hmap.ToSlice() {
		s.Add(entry.Key().(MetaKey))
	}
}

func (s *MetaKeySet) Difference(other *MetaKeySet) *MetaKeySet {
	diff := NewMetaKeySet()
	for _, entry := range s.hmap.ToSlice() {
		if !other.Contains(entry.Key().(MetaKey)) {
			diff.Add(entry.Key().(MetaKey))
		}
	}
	return diff
}

func (s *MetaKeySet) Equals(other *MetaKeySet) bool {
	if s.Len() != other.Len() {
		return false
	}
	for _, entry := range s.hmap.ToSlice() {
		if !other.Contains(entry.Key().(MetaKey)) {
			return false
		}
	}
	return true
}

// func (s *MetaKeySet) Union(other *MetaKeySet) *MetaKeySet {
// union := NewMetaKeySet()
// for _, entry := range s.hmap.ToSlice() {
// union.Add(entry.Key())
// }
// for _, entry := range other.hmap.ToSlice() {
// union.Add(entry.Key())
// }
// return union
// }

// type MetaKeySet struct {
// hmap map[int64][]MetaKeySet
// }

// func NewMetaKeySet() *MetaKeySet {
// return &MetaKeySet(map[int64][]MetaKeySet{})
// }

// func (s *MetaKeySet) Add(k MetaKey) {
// hsh := k.Hash()
// slice := s.hmap[hsh]
// slice = append(slice, k)
// s.hmap[hsh] = slice
// }

// func (s *MetaKeySet) Contains(k MetaKey) bool {
// hsh := k.Hash()
// slice := s.hmap[hsh]
// for _, ki := range slice {
// if k.Equals(ki) {
// return true
// }
// }
// return false
// }

// func (s *MetaKeySet) Keys() []MetaKey {
// allKeys := []MetaKey{}
// for hsh, slice := range s.hmap {
// for _, ki := range slice {
// allKeys = append(allKeys, ki)
// }
// }
// return allKeys
// }

// func (s *MetaKeySet) Union(other *MetaKeySet) *MetaKeySet {
// union := NewMetaKeySet()
// for _, ki := range s.Keys() {
// if other.Contains(ki) {
// union.Add(ki)
// }
// }
// for _, ki := range other.Keys() {
// if s.Contains(ki) {
// union.Add(ki)
// }
// }
// return union
// }

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

// Finds the meta group key when given a template element.
// The template element must be structured as {(M3TA ...) ...}
func findTemplateGroupKey(elem *f.FilterElement, primaryOnly bool) (
	MetaKey, bool, error) {
	ok := false
	var key MetaKey

	if !elem.HasSubElems() || elem.Delims != "{}" {
		return key, ok, nil
	}

	for _, se := range elem.SubElems {
		keyTmp, seOk, err := findMetaGroupKey(se, primaryOnly)
		if err != nil {
			return key, false, err
		}
		if seOk {
			if ok {
				// We've found a second key
				return MetaKey{}, false, fmt.Errorf(
					"Multiple sibling meta keys found in '%s'", elem.FullFilterStr())
			} else {
				key = keyTmp
				ok = true
			}
		}
	}
	return key, ok, nil
}

func isTemplateOrGroup(elem *f.FilterElement, primaryOnly bool) (bool, error) {
	_, ok, err := findTemplateGroupKey(elem, primaryOnly)
	return ok, err
}

func FindPrimaryTemplateGroup(elem *f.FilterElement) (*f.FilterElement, error) {
	var orGroup *f.FilterElement
	nextElem := elem
	for orGroup == nil {
		if !nextElem.HasSubElems() {
			return nil, nil
		} else if nextElem.Delims == "{}" {
			orGroup = nextElem
		} else {
			if len(nextElem.SubElems) > 1 {
				// The OR group is not the single group. It can only be like: (({}))
				return nil, nil
			}
			nextElem = nextElem.SubElems[0]
		}
	}

	util.Assert(orGroup != nil)
	isTemplate, err := isTemplateOrGroup(orGroup, true)
	if err != nil {
		return nil, err
	} else if isTemplate {
		return orGroup, nil
	}
	return nil, nil
}

func FindAllMetaGroupKeys(elem *f.FilterElement) (*MetaKeySet, error) {
	if !elem.HasSubElems() {
		return NewMetaKeySet(), nil
	}

	elemKey, ok, err := findMetaGroupKey(elem, false)
	if err != nil {
		return nil, err
	} else if ok {
		set := NewMetaKeySet()
		set.Add(elemKey)
		return set, nil
	}

	set := NewMetaKeySet()
	for _, se := range elem.SubElems {
		seKeys, err := FindAllMetaGroupKeys(se)
		if err != nil {
			return nil, err
		}
		set.UpdateSet(seKeys)
	}
	return set, nil
}

func normalizedPrimaryElem(primaryElem *f.FilterElement) *f.FilterElement {
	pStr := primaryElem.FullFilterStr()
	pStr = strings.Replace(pStr, META_PRIMARY_LABEL, META_LABEL, -1)
	elemParent, err := f.ParseElement(pStr)
	util.Assert(err == nil, err)
	return elemParent.SubElems[0]
}

func ReplaceMetaGroups(filterElem, primaryElem *f.FilterElement) error {
	if !filterElem.HasSubElems() {
		return nil
	}

	pMetaKey, ok, err := findTemplateGroupKey(primaryElem, true)
	util.Assert(err == nil, err)
	util.Assert(ok)

	for i, se := range filterElem.SubElems {
		k, ok, err := findTemplateGroupKey(se, false)
		if err != nil {
			return err
		}
		if ok && pMetaKey.Equals(k) {
			pCopy := normalizedPrimaryElem(primaryElem)
			pCopy.PreWs = se.PreWs
			pCopy.PostWs = se.PostWs
			filterElem.SubElems[i] = pCopy
		} else {
			ReplaceMetaGroups(se, primaryElem)
		}
	}
	return nil
}

func UpdateMetaGroups(filterElems map[string]*f.FilterElement) error {
	primaryKeyToId := hashmap.NewConcurrentMap()
	primaryKeyToGroup := hashmap.NewConcurrentMap()

	for id, filterElem := range filterElems {
		pGroup, err := FindPrimaryTemplateGroup(filterElem)
		if err != nil {
			return err
		} else if pGroup == nil {
			continue
		}
		pKey, ok, err := findTemplateGroupKey(pGroup, true)
		if err != nil {
			return err
		}
		util.Assert(ok)

		pId, err := primaryKeyToId.Get(pKey)
		util.Assert(err == nil, err)
		if pId != nil {
			return fmt.Errorf("Primary key collision for %v between filters %s, %s",
				pKey, pId.(string), id)
		}

		_, err = primaryKeyToId.Put(pKey, id)
		util.Assert(err == nil, err)
		_, err = primaryKeyToGroup.Put(pKey, pGroup)
		util.Assert(err == nil, err)
	}

	for _, entry := range primaryKeyToId.ToSlice() {
		pKey := entry.Key().(MetaKey)
		id := entry.Value().(string)
		for id2, filterElem := range filterElems {
			if id != id2 {
				// Don't want to try to update the same filter
				pGroup, err := primaryKeyToGroup.Get(pKey)
				util.Assert(err == nil, err)
				ReplaceMetaGroups(filterElem, pGroup.(*f.FilterElement))
			}
		}
	}

	allKeys := NewMetaKeySet()
	for _, filterElem := range filterElems {
		mgks, err := FindAllMetaGroupKeys(filterElem)
		if err != nil {
			return err
		}
		allKeys.UpdateSet(mgks)
	}

	primaryKeys := NewMetaKeySet()
	for _, entry := range primaryKeyToId.ToSlice() {
		key := entry.Key().(MetaKey)
		primaryKeys.Add(key)
	}

	undefinedKeys := allKeys.Difference(primaryKeys)
	if undefinedKeys.Len() > 0 {
		var undefinedKeyStrs []string
		for _, k := range undefinedKeys.Keys() {
			undefinedKeyStrs = append(undefinedKeyStrs, fmt.Sprintf("%v", k))
		}
		return fmt.Errorf("Could not find definition for keys: %s",
			strings.Join(undefinedKeyStrs, ", "))
	}
	return nil
}
