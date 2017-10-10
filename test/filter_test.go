package test

import (
	"fmt"
	"log"
	"testing"

	f "github.com/tsiemens/gmail-tools/filter"
)

func TestCheckDelims(t *testing.T) {
	okStrs := []string{
		"",
		"()",
		"{}",
		"(xx)",
		"zz (xx) yy",
		"(x ())",
		"(x ({({x})} () ))(){(y)}",
	}
	notOkStrs := []string{
		"(",
		"{",
		")",
		"}",
		"((})",
		"(()",
		"())",
		"xx{xx(xx)xxx)xx",
		"\"",
		"\"xxx\" xx \"x ",
	}

	for _, str := range okStrs {
		p := f.NewFullElementParser(str)
		err := p.CheckDelims()
		if err != nil {
			t.Error(err)
		}
	}
	for _, str := range notOkStrs {
		p := f.NewFullElementParser(str)
		err := p.CheckDelims()
		if err == nil {
			t.Errorf("Expected error from \"%s\"", str)
		}
	}
}

var FeDW func(fs string, delims string, preWs string, postWs string,
) *f.FilterElement = f.NewStrFilterElement

func Fe(fs string) *f.FilterElement {
	return FeDW(fs, "", "", "")
}

var SeFeDW func(ses []*f.FilterElement, delims string, preWs string, postWs string,
) *f.FilterElement = f.NewSubElemFilterElement

func SeFe(ses ...*f.FilterElement) *f.FilterElement {
	return SeFeDW(ses, "", "", "")
}

func prs(str string) *f.FilterElement {
	pe, err := f.ParseElement(str)
	if err != nil {
		log.Panicf("Failed to parse \"%s\"", err)
	}
	return pe
}

func prsf(str string) *f.FilterElement {
	return prs(str).SubElems[0]
}

type parseCheckTup struct {
	FilterStr  string
	ExpElement *f.FilterElement
}

func fes(ses ...*f.FilterElement) []*f.FilterElement {
	elms := make([]*f.FilterElement, 0, len(ses))
	for _, e := range ses {
		elms = append(elms, e)
	}
	return elms
}

func checkParse(t *testing.T, filterStr string, exp *f.FilterElement) {
	fmt.Printf("parsing \"%s\"\n", filterStr)
	elm, err := f.ParseElement(filterStr)
	if err != nil {
		t.Fatalf("Error parsing \"%s\": %v\n", filterStr, err)
	} else if !elm.Equals(exp) {
		t.Fatalf("\n%+v (actual) !=\n%+v (exp)\n", elm, exp)
	}
}

func TestParse(t *testing.T) {
	toCheck := []parseCheckTup{
		// Empty
		{"", SeFe(Fe(""))},
		{" ", SeFe(FeDW("", "", " ", ""))},
		{"  ", SeFe(FeDW("", "", "  ", ""))},

		// Single
		{"bla", SeFe(Fe("bla"))},
		{" bla", SeFe(FeDW("bla", "", " ", ""))},
		{"  bla", SeFe(FeDW("bla", "", "  ", ""))},

		// Multiple
		{"foo bar", SeFe(prsf("foo "), prsf("bar"))},
		{" foo  bar ", SeFe(prsf(" foo  "), prsf("bar "))},

		// Group
		{"()", SeFe(SeFeDW(fes(prsf("")), "()", "", ""))},
		{"( )", SeFe(SeFeDW(fes(prsf(" ")), "()", "", ""))},
		{"{x}", SeFe(SeFeDW(fes(prsf("x")), "{}", "", ""))},
		{" {x} ", SeFe(SeFeDW(fes(prsf("x")), "{}", " ", " "))},
		{"{ x y}", SeFe(SeFeDW(fes(prs(" x y").SubElems...), "{}", "", ""))},

		// Multiple groups
		{"{x}(y)", SeFe(prsf("{x}"), prsf("(y)"))},
		{" {x} (y) ", SeFe(prsf(" {x} "), prsf("(y) "))},
		{" x (y) ", SeFe(prsf(" x "), prsf("(y) "))},
		{" (y) x ", SeFe(prsf(" (y) "), prsf("x "))},
		{" x:(y) ", SeFe(prsf(" x:"), prsf("(y) "))},
		{" (y)x", SeFe(prsf(" (y)"), prsf("x"))},

		// Nexted groups
		{"{()}", SeFe(SeFeDW(fes(prsf("()")), "{}", "", ""))},
		{"{(x)}", SeFe(SeFeDW(fes(prsf("(x)")), "{}", "", ""))},

		// Quotes
		{"\"\"", SeFe(FeDW("", "\"\"", "", ""))},
		{"\"(blar)\"", SeFe(FeDW("(blar)", "\"\"", "", ""))},
		{"(\"(blar)\")", SeFe(SeFeDW(fes(prsf("\"(blar)\"")), "()", "", ""))},
	}

	for _, tup := range toCheck {
		checkParse(t, tup.FilterStr, tup.ExpElement)
	}

	// Life-like
	subjectStr := "subject:(\"Fo bar\")"
	checkParse(t, subjectStr, SeFe(
		Fe("subject:"),
		SeFeDW(fes(FeDW("Fo bar", "\"\"", "", "")), "()", "", ""),
	))
	subjectFooBarElemGrp := prs(subjectStr)
	metaFooStr := "{(M3TA label=foo) from:bla@gmail.com} "
	checkParse(t, metaFooStr, SeFe(SeFeDW(
		fes(
			prsf("(M3TA label=foo) "),
			Fe("from:bla@gmail.com")),
		"{}", "", " ",
	)))
	metaFooElem := prsf(metaFooStr)

	tmpFes := append(fes(metaFooElem), subjectFooBarElemGrp.SubElems...)
	checkParse(t,
		"({(M3TA label=foo) from:bla@gmail.com} subject:(\"Fo bar\")) OR Foo",
		SeFe(
			SeFeDW(tmpFes, "()", "", " "),
			prsf("OR "),
			prsf("Foo"),
		),
	)
}
