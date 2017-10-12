package test

import (
	"fmt"
	"testing"

	"github.com/tsiemens/gmail-tools/filter/template"
)

func k(l ...string) template.MetaKey {
	return template.NewMetaKey(l)
}

type findMetaInputTup struct {
	FilterStr string
	ExpKey    template.MetaKey
	ExpOk     bool
	ExpErr    bool
}

func checkFindMetaKey(t *testing.T, tup findMetaInputTup, primaryOnly bool) {
	elem := prsf(tup.FilterStr)
	var key template.MetaKey
	var ok bool
	var err error
	fmt.Println(tup)
	if primaryOnly {
		key, ok, err = template.FindPrimaryMetaGroupKey(elem)
	} else {
		key, ok, err = template.FindMetaGroupKey(elem)
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
