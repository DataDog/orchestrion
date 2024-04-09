package goflags

import (
	"fmt"
	"regexp"
	"strings"
)

// CommandFlags represents the flags provided to a go command invocation
type CommandFlags struct {
	Long  map[string]string
	Short []string
}

// CommandFlagsFromString parses the provided string into a CommandFlags structure
// str should respect the format return by CommandFlags.String()
func CommandFlagsFromString(str string) CommandFlags {
	return ParseCommandFlags(strings.Split(str, " "))
}

// Slice returns the command flags as a string slice
// - Long flags are returned as a string of the form '-flagName="flagVal"'
// - short flags are returned as a string of the form '-flagName'
func (f CommandFlags) Slice() []string {
	flags := make([]string, 0, len(f.Long)+len(f.Short))
	for flag, val := range f.Long {
		flags = append(flags, fmt.Sprintf("%s=\"%s\"", flag, val))
	}
	for _, flag := range f.Short {
		flags = append(flags, flag)
	}
	return flags
}

// String returns a single string of the concatenated flags
func (f CommandFlags) String() string {
	return strings.Join(f.Slice(), " ")
}

// ParseCommandFlags parses a slice representing a go command invocation
// and returns its flags. Direct arguments to the command are ignored
func ParseCommandFlags(args []string) CommandFlags {
	flags := CommandFlags{
		Long:  make(map[string]string, len(args)),
		Short: make([]string, 0, len(args)),
	}

	for i := 0; i < len(args); i += 1 {
		arg := args[i]
		if !isOption(arg) {
			continue
		}
		if isAssigned(arg) {
			key, val, found := strings.Cut(arg, "=")
			if found {
				flags.Long[key] = val
			}
		} else if i == len(args)-1 || isOption(args[i+1]) {
			flags.Short = append(flags.Short, arg)
		} else {
			flags.Long[arg] = args[i+1]
			i = i + 1
		}
	}

	return flags
}

func isOption(str string) bool {
	return len(str) > 0 && str[0] == '-'
}

func isAssigned(str string) bool {
	return regexp.MustCompile(".+=.+").MatchString(str)
}
