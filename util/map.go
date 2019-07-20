package util

import "sort"

func StrStrMapKeys(strStrMap map[string]string) []string {
	keys := make([]string, 0, len(strStrMap))
	for k := range strStrMap {
		keys = append(keys, k)
	}
	return keys
}

func StrBoolMapKeys(strStrMap map[string]bool) []string {
	keys := make([]string, 0, len(strStrMap))
	for k := range strStrMap {
		keys = append(keys, k)
	}
	return keys
}

func SortStrSlice(slice []string) []string {
	sort.Strings(slice)
	return slice
}
