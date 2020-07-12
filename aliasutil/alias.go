package aliasutil

import (
	"log"
	"regexp"
	"strconv"

	"github.com/google/shlex"

	"github.com/tsiemens/gmail-tools/util"
)

type ParsedArgs struct {
	UnknownArgs []string
	// Element 0 will ALWAYS be "--"
	PosArgs []string
}

func isFlagArg(arg string) bool {
	return ((len(arg) >= 3 && arg[1] == '-') ||
		(len(arg) >= 2 && arg[0] == '-' && arg[1] != '-'))
}

// Returns unknown args, positional args
func ClassifyArgs(args []string) ParsedArgs {
	pa := ParsedArgs{[]string{}, []string{}}

	posStarted := false
	foundFlag := false
	for _, arg := range args {
		if arg == "--" {
			if !posStarted {
				posStarted = true
			}
		} else if !posStarted && isFlagArg(arg) {
			foundFlag = true
		}

		if !foundFlag || posStarted {
			pa.PosArgs = append(pa.PosArgs, arg)
		} else {
			// While we are in 'flag territory', we don't know which are args for flags
			// and which are positional arguments.
			pa.UnknownArgs = append(pa.UnknownArgs, arg)
		}
	}

	return pa
}

// providedArgs: Arguments provided to the command, minus the command name itself
// aliasTargetStr: The command string provided, specifying what to alias to.
//                 May include special characters to specify arguments provided to
//                 the alias. By default, extra args are placed at the end of the alias.
//
// Alias target special strings are:
// $1,$2... - Insert a positional argument here.
// $R - Insert remaining unused arguments here.
func CreateAliasArgs(providedArgs []string, aliasTargetStr string) ([]string, error) {
	util.Debugln("CreateAliasArgs:", providedArgs, aliasTargetStr)
	targetArgs, err := shlex.Split(aliasTargetStr)
	if err != nil {
		return nil, err
	}

	aliasArgs := []string{}

	posArgPat := regexp.MustCompile(`(^|[^\\])\$(\d+)`)
	usedPosArgs := map[int]bool{}
	inArgs := ClassifyArgs(providedArgs)

	remainderInsertIndex := -1

	for _, arg := range targetArgs {
		matches := posArgPat.FindAllStringSubmatch(arg, -1)
		if matches != nil {
			usedPosArgsInArg := map[int]bool{}

			for _, m := range matches {
				index, err := strconv.Atoi(m[2])
				if err != nil {
					log.Fatalln("Error parsing alias: Unable to parse index", m[2], err)
				}
				if _, ok := usedPosArgsInArg[index]; ok {
					continue
				}

				specificPosArgPat := regexp.MustCompile(`(^|[^\\])\$` + m[2])
				inArg := ""
				realIndex := index - 1
				if realIndex >= 0 && realIndex < len(inArgs.PosArgs) {
					inArg = inArgs.PosArgs[realIndex]
				}
				arg = specificPosArgPat.ReplaceAllString(arg, "${1}"+inArg)
				usedPosArgs[realIndex] = true
				usedPosArgsInArg[realIndex] = true
			}
		}

		if arg == "$R" {
			remainderInsertIndex = len(aliasArgs)
		}
		aliasArgs = append(aliasArgs, arg)
	}

	remainderArgs := []string{}
	for _, arg := range inArgs.UnknownArgs {
		remainderArgs = append(remainderArgs, arg)
	}

	for i, arg := range inArgs.PosArgs {
		if _, ok := usedPosArgs[i]; !ok {
			remainderArgs = append(remainderArgs, arg)
		}
	}

	if remainderInsertIndex != -1 {
		beforeRemainder := make([]string, remainderInsertIndex, len(aliasArgs)-1+len(remainderArgs))
		copy(beforeRemainder, aliasArgs[:remainderInsertIndex])
		afterRemainder := make([]string, len(aliasArgs)-remainderInsertIndex-1)
		copy(afterRemainder, aliasArgs[remainderInsertIndex+1:])
		aliasArgs = append(beforeRemainder, remainderArgs...)
		aliasArgs = append(aliasArgs, afterRemainder...)
	} else {
		aliasArgs = append(aliasArgs, remainderArgs...)
	}

	return aliasArgs, nil
}
