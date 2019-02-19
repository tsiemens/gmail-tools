package api

import "regexp"

var fromFieldRegexp = regexp.MustCompile(`\s*(\S|\S.*\S)\s*<.*>\s*`)

func GetFromName(fromHeaderVal string) string {
	matches := fromFieldRegexp.FindStringSubmatch(fromHeaderVal)
	if len(matches) > 0 {
		return matches[1]
	}
	return fromHeaderVal
}
