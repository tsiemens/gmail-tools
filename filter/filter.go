package filter

import (
	"fmt"
	"strings"

	"github.com/golang-collections/collections/stack"

	"github.com/tsiemens/gmail-tools/prnt"
	"github.com/tsiemens/gmail-tools/util"
)

type FilterElement struct {
	// FilterStr and SubElems are mutually exclusive
	FilterStr string
	SubElems  []*FilterElement

	// If not empty string, must be a 2 character string
	Delims string
	// Whitespace
	PreWs  string
	PostWs string
}

func NewStrFilterElement(
	fs string, delims string, preWs string, postWs string) *FilterElement {
	return &FilterElement{FilterStr: fs, Delims: delims, PreWs: preWs, PostWs: postWs}
}

func NewSubElemFilterElement(
	ses []*FilterElement, delims string, preWs string, postWs string) *FilterElement {
	return &FilterElement{SubElems: ses, Delims: delims, PreWs: preWs, PostWs: postWs}
}

func (e *FilterElement) checkConsistency() {
	util.Assertf(
		!(e.FilterStr != "" && len(e.SubElems) > 0),
		"Found inconsistent FilterStr (\"%s\")/SubElems (%v)\n",
		e.FilterStr, e.SubElems)
	util.Assertf(e.Delims == "" || len(e.Delims) == 2,
		"Invalid demims: '%s'\n", e.Delims)
}

func (e *FilterElement) Equals(other *FilterElement) bool {
	if other == nil ||
		other.FilterStr != e.FilterStr ||
		other.Delims != e.Delims ||
		other.PreWs != e.PreWs ||
		other.PostWs != e.PostWs ||
		len(other.SubElems) != len(e.SubElems) {
		return false
	}

	for i, se := range e.SubElems {
		if !se.Equals(other.SubElems[i]) {
			return false
		}
	}
	return true
}

func (e *FilterElement) HasSubElems() bool {
	e.checkConsistency()
	return len(e.SubElems) > 0
}

func (e *FilterElement) maybeWrapIng(str string) string {
	if e.Delims != "" {
		return string(e.Delims[0]) + str + string(e.Delims[1])
	}
	return str
}

func (e *FilterElement) maybeWrapIngAndPad(str string) string {
	str = e.maybeWrapIng(str)
	return e.PreWs + str + e.PostWs
}

func (e *FilterElement) FullFilterStr() string {
	e.checkConsistency()
	if e.HasSubElems() {
		subElemStrs := make([]string, len(e.SubElems))
		for _, subElem := range e.SubElems {
			subElemStrs = append(subElemStrs, subElem.FullFilterStr())
		}
		return e.maybeWrapIngAndPad(strings.Join(subElemStrs, ""))
	}
	return e.maybeWrapIngAndPad(e.FilterStr)
}

func (e *FilterElement) StringCustomLines(showPtr bool, indent int, singleLine bool,
) []string {
	indented := func(str string, extra int) string {
		if singleLine {
			return str
		}
		return strings.Repeat(" ", indent+extra) + str
	}

	var reprLines []string
	reprStr := indented("FilterElement", 0)
	if showPtr {
		reprStr += fmt.Sprintf("[%p]", e)
	}
	reprStr += "{"

	appendStrAttr := func(attrName, attr string, maybe bool) {
		if !maybe || attr != "" {
			reprStr += fmt.Sprintf("%s:\"%s\" ", attrName, attr)
		}
	}

	appendStrAttr("FilterStr", e.FilterStr, len(e.SubElems) > 0)
	appendStrAttr("Delims", e.Delims, true)
	appendStrAttr("PreWs", e.PreWs, true)
	appendStrAttr("PostWs", e.PostWs, true)

	if len(e.SubElems) > 0 {
		reprStr += "SubElems:{"
		var allSeLines []string
		for _, se := range e.SubElems {
			var seLines []string
			if se == nil {
				seLines = []string{"<nil>,"}
			} else {
				seLines = se.StringCustomLines(showPtr, indent+2, singleLine)
				seLines[len(seLines)-1] += ","
			}
			allSeLines = append(allSeLines, seLines...)
		}

		if singleLine {
			reprStr += strings.Join(allSeLines, "") + "}"
		} else {
			reprLines = append(reprLines, reprStr)
			reprLines = append(reprLines, allSeLines...)
			reprStr = indented("}", 2)
		}
	}

	reprStr += "}"
	reprLines = append(reprLines, reprStr)
	return reprLines
}

func (e *FilterElement) StringCustom(showPtr bool, indent int, singleLine bool,
) string {
	return strings.Join(e.StringCustomLines(showPtr, indent, singleLine), "\n")
}

// Object representation
func (e *FilterElement) String() string {
	return e.StringCustom(false, 0, true)
}

// Delimiters
const (
	PARENS = "()"
	BRACES = "{}"

	OPEN_DELIMS  = "({"
	CLOSE_DELIMS = ")}"
)

// Set in init
var DELIM_PAIRS []string

type ParseMode int

const (
	PRE_TEXT_MODE int = iota
	TEXT_MODE
	POST_TEXT_MODE
	QUOTED_TEXT_MODE
)

type ElementParser struct {
	filterStr string
	startIdx  int
	currIdx   int
	lastIdx   int
}

func NewElementParser(filterStr string, startIdx int, lastIdx int) *ElementParser {
	return &ElementParser{filterStr: filterStr, startIdx: startIdx, currIdx: startIdx,
		lastIdx: lastIdx}
}

func NewFullElementParser(filterStr string) *ElementParser {
	return NewElementParser(filterStr, 0, len(filterStr)-1)
}

func cIn(str string, c byte) bool {
	return strings.Contains(str, string(c))
}

func cIndex(str string, c byte) int {
	return strings.Index(str, string(c))
}

func (p *ElementParser) lastGroupIdx(groupStart int) (int, error) {
	delimStack := stack.New()
	i := groupStart
	if !cIn(OPEN_DELIMS, p.filterStr[i]) {
		prnt.StderrLog.Panicln("groupStart was not an open delimiter")
	}

	retIndex := 0
	for i := groupStart; true; i++ {
		c := p.filterStr[i]
		if cIn(OPEN_DELIMS, c) {
			delimStack.Push(c)
		} else if cIn(CLOSE_DELIMS, c) {
			di := strings.Index(CLOSE_DELIMS, string(c))
			if delimStack.Len() == 0 || delimStack.Peek().(byte) != OPEN_DELIMS[di] {
				return 0, fmt.Errorf("Mismatched closing delim at index %d", i)
			} else {
				delimStack.Pop()
			}
		}

		if delimStack.Len() == 0 {
			retIndex = i
			break
		}
	}
	return retIndex, nil
}

// Returns group, last delimiter index
func (p *ElementParser) parseDelimitedGroup(startIdx int) (*FilterElement, int, error) {
	lastDelimIdx, err := p.lastGroupIdx(startIdx)
	if err != nil {
		return nil, 0, err
	} else if lastDelimIdx == 0 {
		prnt.StderrLog.Panicln("lastDelimIdx was 0")
	}

	subParser := NewElementParser(p.filterStr, startIdx+1, lastDelimIdx-1)
	groupElem, err := subParser.Parse()
	if err != nil {
		return nil, 0, err
	}
	if p.filterStr[startIdx] == '(' {
		groupElem.Delims = "()"
	} else {
		groupElem.Delims = "{}"
	}

	return groupElem, lastDelimIdx, nil
}

func (p *ElementParser) parseNext() (*FilterElement, error) {
	util.Debugln("ElementParser.parseNext")
	mode := PRE_TEXT_MODE
	delims := ""
	substr := ""
	var groupElem *FilterElement = nil
	preWs := ""
	postWs := ""
	isFirstParse := p.startIdx == p.currIdx
	var err error = nil
	breakIter := false

	i := p.currIdx
	util.Debugf("parseNext: currIdx: %d filterStr: '%s'\n", i, p.filterStr)
	for ; i <= p.lastIdx; i++ {
		switch mode {
		case PRE_TEXT_MODE:
			if p.filterStr[i] == ' ' {
				preWs += " "
			} else if cIn(OPEN_DELIMS, p.filterStr[i]) {
				groupElem, i, err = p.parseDelimitedGroup(i)
				if err != nil {
					return nil, err
				}
				mode = POST_TEXT_MODE
			} else if cIn(CLOSE_DELIMS, p.filterStr[i]) {
				prnt.StderrLog.Panicf(
					"Found unmatched close delim while parsing: idx %d\n", i)
			} else if p.filterStr[i] == '"' {
				mode = QUOTED_TEXT_MODE
				delims = "\"\""
				substr = ""
			} else {
				mode = TEXT_MODE
				substr = string(p.filterStr[i])
			}
		case TEXT_MODE:
			if p.filterStr[i] == ' ' {
				mode = POST_TEXT_MODE
				postWs += " "
			} else if cIn(OPEN_DELIMS, p.filterStr[i]) {
				// This group will be next
				breakIter = true
			} else if cIn(CLOSE_DELIMS, p.filterStr[i]) {
				prnt.StderrLog.Panicf(
					"Found unmatched close delim while parsing: idx %d\n", i)
			} else if p.filterStr[i] == '"' {
				// This quoted element will be next
				breakIter = true
			} else {
				substr += string(p.filterStr[i])
			}
		case POST_TEXT_MODE:
			if p.filterStr[i] == ' ' {
				postWs += " "
			} else {
				breakIter = true
			}
		case QUOTED_TEXT_MODE:
			if p.filterStr[i] == '"' {
				i++
				if len(p.filterStr) > i && p.filterStr[i] == ' ' {
					// There is whitespace after the closing delim. Count it as part
					// of this element.
					mode = POST_TEXT_MODE
					i--
				} else {
					breakIter = true
				}
			} else {
				substr += string(p.filterStr[i])
			}
		default:
			prnt.StderrLog.Fatalf("Invalid mode: %v\n", mode)
		}
		if breakIter {
			break
		}
	}

	p.currIdx = i
	if substr == "" && groupElem == nil {
		if isFirstParse {
			if p.currIdx == p.startIdx {
				// In the event that the filter string was length 0, bump the index
				// past the end
				p.currIdx = p.startIdx + 1
			}
		} else {
			if len(preWs) != 0 || len(postWs) != 0 {
				prnt.StderrLog.Panicln("Found whitespace in nil (final) element)")
			}
			// We are done parsing
			return nil, nil
		}
	}

	var elm *FilterElement = nil
	if groupElem != nil {
		elm = groupElem
		elm.PreWs = preWs
		elm.PostWs = postWs
	} else {
		elm = NewStrFilterElement(substr, delims, preWs, postWs)
	}
	return elm, nil
}

func (p *ElementParser) Parse() (*FilterElement, error) {
	util.Debugln("ElementParser.Parse")
	err := p.CheckDelims()
	if err != nil {
		return nil, err
	}

	var filterElements []*FilterElement
	done := false
	for !done {
		nextElem, err := p.parseNext()
		if err != nil {
			return nil, err
		} else if nextElem == nil {
			done = true
		} else {
			filterElements = append(filterElements, nextElem)
		}
	}

	return NewSubElemFilterElement(filterElements, "", "", ""), nil
}

// Return a string indicating where the error is under the filter string
func (p *ElementParser) parseErrorStr(index int) string {
	return p.filterStr + "\n" + strings.Repeat(" ", index) + "^"
}

func (p *ElementParser) CheckDelims() error {
	delimStack := stack.New()
	for i := 0; i < len(p.filterStr); i++ {
		c := p.filterStr[i]
		if cIn(OPEN_DELIMS, c) {
			delimStack.Push(c)
		} else if cIn(CLOSE_DELIMS, c) {
			di := cIndex(CLOSE_DELIMS, c)
			if delimStack.Len() == 0 || delimStack.Peek().(byte) != OPEN_DELIMS[di] {
				return fmt.Errorf("Mismatched closing delim:\n%s", p.parseErrorStr(i))
			} else {
				delimStack.Pop()
			}
		}
	}

	if delimStack.Len() != 0 {
		return fmt.Errorf("Unmatched delim:\n%s", p.parseErrorStr(len(p.filterStr)-1))
	}

	if (strings.Count(p.filterStr, "\"") % 2) != 0 {
		return fmt.Errorf("Unmatched quote")
	}
	return nil
}

func ParseElement(filterStr string) (*FilterElement, error) {
	p := NewFullElementParser(filterStr)
	return p.Parse()
}

func init() {
	DELIM_PAIRS = []string{PARENS, BRACES}
}
