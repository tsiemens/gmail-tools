package test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/tsiemens/gmail-tools/filter"
	tmp "github.com/tsiemens/gmail-tools/filter/template"
	"github.com/tsiemens/gmail-tools/util"
)

func k(l ...string) tmp.MetaKey {
	return tmp.NewMetaKey(l)
}

type findMetaInputTup struct {
	FilterStr string
	ExpKey    tmp.MetaKey
	ExpOk     bool
	ExpErr    bool
}

func checkFindMetaKey(t *testing.T, tup findMetaInputTup, primaryOnly bool) {
	elem := prsf(tup.FilterStr)
	var key tmp.MetaKey
	var ok bool
	var err error
	fmt.Println(tup)
	if primaryOnly {
		key, ok, err = tmp.FindPrimaryMetaGroupKey(elem)
	} else {
		key, ok, err = tmp.FindMetaGroupKey(elem)
	}
	if tup.ExpErr {
		if err == nil || ok {
			t.Errorf("Expected error. ok: %v, err: %v", ok, err)
		}
	}
	if ok != tup.ExpOk {
		t.Errorf("Expected ok: %v. found %v", tup.ExpOk, ok)
	}
	if tup.ExpOk {
		if !tup.ExpKey.Equals(key) {
			t.Errorf("Expected key: %v, found %v", tup.ExpKey, key)
		}
	}
	if t.Failed() {
		t.FailNow()
	}
}

func TestFindMetaKey(t *testing.T) {
	inputs := []findMetaInputTup{
		{"(xxx)", k(), false, false},
		{"(M3TA xxx)", k("xxx"), true, false},
		{"(xxx M3TA bla)", k("bla", "xxx"), true, false},
		{"(bla M3TA xxx)", k("bla", "xxx"), true, false},
		{"(xxx M3TA (bla))", k("(bla)", "xxx"), true, false},
		{"(M3TA)", k(), false, true},
		{"M3TA xxx", k(), false, false},
		{"\"M3TA xxx bla\"", k("bla", "xxx"), true, false},
		{"\"xxx M3TA\"", k("xxx"), true, false},
		{"\"M3TA\"", k(), false, false},
	}

	for _, tup := range inputs {
		checkFindMetaKey(t, tup, false)
	}

	checkFindMetaKey(t, findMetaInputTup{"(M3TAP xxx)", k("xxx"), true, false}, true)
	checkFindMetaKey(t, findMetaInputTup{"(M3TA xxx)", k("xxx"), false, false}, true)
}

type findPrimaryGroupInputTup struct {
	FilterStr string
	ExpElem   *filter.FilterElement
	ExpErr    bool
}

func TestFindPrimaryGroup(t *testing.T) {
	inputs := []findPrimaryGroupInputTup{
		{"{}", nil, false},
		{"{(M3TA bla)}", nil, false},
		{"{(M3TAP bla)}", prsf("{(M3TAP bla)}"), false},
		{"{xx (M3TAP bla)}", prsf("{xx (M3TAP bla)}"), false},
		{"{(M3TAP bla) yy}", prsf("{(M3TAP bla) yy}"), false},
		{"({(M3TAP bla) yy})", prsf("{(M3TAP bla) yy}"), false},
		{"{(M3TAP bla) yy (M3TAP foo)}", nil, true},
	}

	for _, tup := range inputs {
		fmt.Println(tup)
		filterElem := prs(tup.FilterStr)
		pElem, err := tmp.FindPrimaryTemplateGroup(filterElem)
		if tup.ExpErr {
			if err == nil {
				t.Fatalf("Expected error, got <nil>")
			}
		} else {
			if err != nil {
				t.Fatalf("Expected nil error, got %v", err)
			} else {
				assertFiltersEqual(t, pElem, tup.ExpElem)
			}
		}
	}
}

func TestReplace(t *testing.T) {
	primary := "{(M3TAP foo) (thing) thing2}"
	primaryGrp, err := tmp.FindPrimaryTemplateGroup(prs(primary))
	util.Assert(err == nil, err)
	primary = strings.Replace(primary, "M3TAP", "M3TA", -1)

	check := func(filterStr, newFilterStr string) {
		fmt.Printf("TestReplace: \"%s\", \"%s\"\n", filterStr, newFilterStr)
		filterGroup := prs(filterStr)
		err := tmp.ReplaceMetaGroups(filterGroup, primaryGrp)
		if err != nil {
			t.Fatalf("err was not nil: %v", err)
		} else if filterGroup.FullFilterStr() != newFilterStr {
			t.Fatalf("\"%s\" != \"%s\"", filterGroup.FullFilterStr(), newFilterStr)
		}
	}

	check("something {( fo) x}", "something {( fo) x}")
	check("something {(M3TA fo) x}", "something {(M3TA fo) x}")
	check("something {(M3TA foo) x}", "something "+primary)
	check("something {(M3TA foo) x} ({(foo M3TA) y})",
		fmt.Sprintf("something %s (%s)", primary, primary))
}

func mk(labels ...string) tmp.MetaKey {
	return tmp.NewMetaKey(labels)
}

var ks func(keys ...tmp.MetaKey) *tmp.MetaKeySet = tmp.NewMetaKeySet

func TestFindAllKeys(t *testing.T) {
	check := func(filterStr string, expSet *tmp.MetaKeySet) {
		fmt.Printf("TestFindAllKeys \"%s\"\n", filterStr)
		keySet, err := tmp.FindAllMetaGroupKeys(prs(filterStr))
		if err != nil {
			t.Fatal("Found error", err)
		}
		if !keySet.Equals(expSet) {
			t.Fatalf("Key sets not equal: %v (actual), %v (expected)",
				keySet.Keys(), expSet.Keys())
		}
	}

	check("", ks())
	check("{(M3TA foo) x}", ks(mk("foo")))
	check("{(M3TA foo) {(M3TA bar) x}}", ks(mk("foo"), mk("bar")))
	check("{(M3TA foo) x} b {(M3TA bar) y}", ks(mk("foo"), mk("bar")))
	check("{(M3TAP foo) x} b {(M3TA bar) y}", ks(mk("foo"), mk("bar")))
}

func assertFilterMapsEqual(t *testing.T, actual map[string]*filter.FilterElement,
	exp map[string]*filter.FilterElement) {

	if actual == nil {
		t.Fatal("Actual was <nil>")
	}
	for id, elem := range actual {
		if expVal, ok := exp[id]; ok {
			if !expVal.Equals(elem) {
				t.Errorf("\n+? %s: %v\n-?%s: %v\n", id, elem, id, expVal)
			}
		} else {
			t.Errorf("\n+? %s: %v\n", id, elem)
		}
	}
	for id, expElem := range exp {
		if _, ok := actual[id]; !ok {
			t.Errorf("\n-? %s: %v\n", id, expElem)
		}
	}
	if t.Failed() {
		t.FailNow()
	}
}

func TestUpdateAll(t *testing.T) {
	filters := map[string]*filter.FilterElement{
		"1": prs("{(M3TAP foo) new}"),
		"2": prs("bla {(M3TA foo) old} x"),
	}

	err := tmp.UpdateMetaGroups(filters)
	if err != nil {
		t.Fatal("Found error", err)
	}
	expected := map[string]*filter.FilterElement{
		"1": prs("{(M3TAP foo) new}"),
		"2": prs("bla {(M3TA foo) new} x"),
	}
	assertFilterMapsEqual(t, filters, expected)

	// Other with root key
	filters = map[string]*filter.FilterElement{
		"1": prs("{(M3TAP foo) new}"),
		"2": prs("{(M3TA foo) old}"),
	}
	err = tmp.UpdateMetaGroups(filters)
	if err != nil {
		t.Fatal("Found error", err)
	}
	expected = map[string]*filter.FilterElement{
		"1": prs("{(M3TAP foo) new}"),
		"2": prs("{(M3TA foo) new}"),
	}
	assertFilterMapsEqual(t, filters, expected)

	// updates within primaries
	filters = map[string]*filter.FilterElement{
		"1": prs("{(M3TAP bar) newbarthing}"),
		"2": prs("{(M3TAP foo) newfoothing {(M3TA bar) oldbarthing}}"),
		"3": prs("xx {(M3TA foo) oldfoothig {(M3TA bar) olderbarthing}}"),
	}

	err = tmp.UpdateMetaGroups(filters)
	if err != nil {
		t.Fatal("Found error", err)
	}
	expected = map[string]*filter.FilterElement{
		"1": prs("{(M3TAP bar) newbarthing}"),
		"2": prs("{(M3TAP foo) newfoothing {(M3TA bar) newbarthing}}"),
		"3": prs("xx {(M3TA foo) newfoothing {(M3TA bar) newbarthing}}"),
	}
	assertFilterMapsEqual(t, filters, expected)

	// ----------Errors-------------------
	// Duplicate primaries
	filters = map[string]*filter.FilterElement{
		"1": prs("{(M3TAP foo) new}"),
		"2": prs("{(M3TAP foo) old}"),
	}
	err = tmp.UpdateMetaGroups(filters)
	if err == nil {
		t.Fatal("Expected non-nil error")
	}

	// No labels
	filters = map[string]*filter.FilterElement{
		"1": prs("{(M3TA) new}"),
	}
	err = tmp.UpdateMetaGroups(filters)
	if err == nil {
		t.Fatal("Expected non-nil error")
	}

	filters = map[string]*filter.FilterElement{
		"1": prs("{(M3TAP) new}"),
	}
	err = tmp.UpdateMetaGroups(filters)
	if err == nil {
		t.Fatal("Expected non-nil error")
	}

	// Missing key
	filters = map[string]*filter.FilterElement{
		"1": prs("{(M3TAP foo) new}"),
		"2": prs("x {(M3TA bar) old} {(M3TA baz) x}"),
	}
	err = tmp.UpdateMetaGroups(filters)
	if err == nil {
		t.Fatal("Expected non-nil error")
	}
}
