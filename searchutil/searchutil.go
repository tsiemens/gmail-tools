package searchutil

import (
	"sort"

	"github.com/tsiemens/gmail-tools/prnt"
	"github.com/tsiemens/gmail-tools/util"
)

type CountedString struct {
	Str   string
	Count int
}

func MapToSortedCountedStrings(countMap map[string]int) []CountedString {
	stringsSorted := make([]CountedString, 0, len(countMap))
	for data, count := range countMap {
		stringsSorted = append(stringsSorted, CountedString{data, count})
	}
	sort.Slice(
		stringsSorted,
		func(i, j int) bool { return stringsSorted[i].Count > stringsSorted[j].Count })

	return stringsSorted
}

type CountedStringDefaultMap struct {
	Map map[string]int
}

func NewCountedStringDefaultMap() *CountedStringDefaultMap {
	return &CountedStringDefaultMap{make(map[string]int)}
}

func (m *CountedStringDefaultMap) Inc(key string) {
	var count int
	var ok bool
	if count, ok = m.Map[key]; !ok {
		count = 0
	}
	count++
	m.Map[key] = count
}

// header: Any string
// skippedFmt: a format string with a single %d in it
// threshPercent: Limit shown entries to be above x% of the first 3
//                larges entries. 0 to show all.
func PrintCountsWithThresholdOfMax(header string, skippedNoun string,
	minShown int, threshPercent int, countMap map[string]int) {

	sortedTups := MapToSortedCountedStrings(countMap)
	prnt.Hum.Always.Ln(header)

	countThreshold := 0
	if len(sortedTups) > 0 {
		largestFewCnt := util.IntMin(len(sortedTups), 3)
		largestFewTotal := 0
		for i := 0; i < largestFewCnt; i++ {
			largestFewTotal += sortedTups[i].Count
		}
		largestFewAvg := largestFewTotal / largestFewCnt

		// Limit to x% of biggest few
		countThreshold = int(float32(largestFewAvg) * float32(threshPercent) / 100.0)
	}
	skippedCnt := 0
	for i, tup := range sortedTups {
		if i < minShown ||
			tup.Count >= countThreshold {
			prnt.Hum.Always.F("%-40s %d\n", tup.Str, tup.Count)
		} else {
			skippedCnt++
		}
	}
	if skippedCnt > 0 {
		prnt.Hum.Always.F("(%d additional %s skipped; counts too low)\n",
			skippedCnt, skippedNoun)

	}
}
