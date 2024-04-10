package goflags

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/datadog/orchestrion/internal/goproxy"
	"github.com/shirou/gopsutil/v3/process"
)

// CommandFlags represents the flags provided to a go command invocation
type CommandFlags struct {
	Long  map[string]string
	Short []string
}

func (f CommandFlags) Trim(flags ...string) {
	for _, flag := range flags {
		delete(f.Long, flag)
		for i, fl := range f.Short {
			if fl == flag {
				f.Short = append(f.Short[:i], f.Short[i+1:]...)
				break
			}
		}
	}
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

// Flags return the top level go command flags
func Flags() (CommandFlags, error) {
	var err error
	if flags == nil {
		*flags, err = parentGoCommandFlags()
		flags.Trim("-toolexec", "-o")
	}

	return *flags, err
}

func isOption(str string) bool {
	return len(str) > 0 && str[0] == '-'
}

func isAssigned(str string) bool {
	return regexp.MustCompile(".+=.+").MatchString(str)
}

// parentGoCommandFlags backtracks through the process tree
// to find a parent go command invocation and returns its arguments
func parentGoCommandFlags() (flags CommandFlags, err error) {
	goBin, err := goproxy.GoBin()
	if err != nil {
		return flags, err
	}

	p, err := process.NewProcess(int32(os.Getpid()))
	if err != nil {
		return flags, err
	}

	// Backtrack through the process stack until we find the parent Go command
	var args []string
	for {
		p, err = p.Parent()
		if err != nil {
			return flags, err
		}
		args, err = p.CmdlineSlice()
		if err != nil {
			return flags, err
		}
		cmd, err := exec.LookPath(args[0])
		if err != nil {
			return flags, err
		}
		// Found the go command process, break out of backtracking
		if cmd == goBin {
			break
		}
	}

	return ParseCommandFlags(args), nil
}

var flags *CommandFlags
