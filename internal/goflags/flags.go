// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

// Package goflags allows parsing go command invocations and storing their flags in a
// CommandFlags structure. It also provides utilities to backtrack through the process stack to
// find and parse the flags of the first parent go command found in the process hierarchy.
package goflags

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/DataDog/orchestrion/internal/goenv"
	"github.com/DataDog/orchestrion/internal/goflags/quoted"
	"github.com/DataDog/orchestrion/internal/log"
	"github.com/shirou/gopsutil/v3/process"
	"golang.org/x/tools/go/packages"
)

// CommandFlags represents the flags provided to a go command invocation
type CommandFlags struct {
	Long    map[string]string
	Short   map[string]struct{}
	Unknown []string // flags we don't process but store anyway
}

var (
	shortFlags = map[string]struct{}{
		"-asan":       {}, // Enables address sanitizer
		"-cover":      {}, // Enables coverage collection
		"-linkshared": {}, // Build code that links against shared libraries
		"-modcacherw": {}, // Keep module cache files read-write
		"-msan":       {}, // Enable memory sanitizer
		"-race":       {}, // Enable data race detection
		"-trimpath":   {}, // Remove all file system paths from the resulting executable
		"-work":       {}, // Keep working temporary directory instead of deleting it
	}
	longFlags = map[string]struct{}{
		"-asmflags":   {}, // Flags passed through to the assembly
		"-buildmode":  {}, // Set build mode
		"-buildvcs":   {}, // Whether to stamp binaries with version control information
		"-compiler":   {}, // Select what compiler to use
		"-covermode":  {}, // Set coverage mode
		"-coverpkg":   {}, // Set list of packages to collect coverage for
		"-gccgoflags": {}, // Flags passed through to the gccgo compiler
		"-gcflags":    {}, // Flags passed through to the gc compiler
		"-ldflags":    {}, // Flags passed through to the linker
		"-mod":        {}, // Set module download mode
		"-modfile":    {}, // Set module file
		"-overlay":    {}, // Set overlay file
		"-pgo":        {}, // Set profile-guided optimization profile file
		"-pkgdir":     {}, // Set package install & load directory
		"-tags":       {}, // Set build tags
	}
)

// Get returns the value of the specified long-form flag if present. The name is
// provided including the leading hyphen, e.g: "-tags".
func (f CommandFlags) Get(flag string) (val string, found bool) {
	val, found = f.Long[flag]
	return
}

// Trim removes the specified flags and their value from the long and short flags
func (f CommandFlags) Trim(flags ...string) {
	for _, flag := range flags {
		delete(f.Long, flag)
		delete(f.Short, flag)
	}
}

// Slice returns the command flags as a string slice
// - long flags are returned as a string of the form '-flagName="flagVal"'
// - short flags are returned as a string of the form '-flagName'
// - unknow flags and values are ignored
func (f CommandFlags) Slice() []string {
	flags := make([]string, 0, len(f.Long)+len(f.Short))
	for flag, val := range f.Long {
		flags = append(flags, fmt.Sprintf("%s=%s", flag, val))
	}
	for flag := range f.Short {
		flags = append(flags, flag)
	}
	return flags
}

// ParseCommandFlags parses a slice representing a go command invocation
// and returns its flags. Direct arguments to the command are ignored. The value
// of $GOFLAGS is also included in the returned flags.
func ParseCommandFlags(wd string, args []string) (CommandFlags, error) {
	flags := CommandFlags{
		Long:  make(map[string]string, len(longFlags)),
		Short: make(map[string]struct{}, len(shortFlags)),
	}

	goflags := os.Getenv("GOFLAGS")
	goflagsArgs, err := quoted.Split(goflags)
	if err != nil {
		log.Warnf("Failed to interpreted quoted strings in GOFLAGS=%q: %v\n", goflags, err)
	} else {
		log.Tracef("GOFLAGS arguments: %q\n", goflagsArgs)
	}

	// Remove any `-C` flag provided on the command line. This is required to immediately follow the `go` command, and
	// can be present only once.
	if len(args) > 0 {
		if arg := args[0]; strings.HasPrefix(arg, "-C") {
			if arg == "-C" && len(args) > 1 {
				// ["-C", "directory", ...]
				log.Tracef("Skipping -C %q flag\n", args[1])
				args = args[2:]
			} else if arg[:3] == "-C=" {
				// ["-C=directory", ...]
				log.Tracef("Skipping %q flag\n", arg)
				args = args[1:]
			}
		}
	}

	// The next argument after a `-C` (if present) would be the go command name ("run", "test", "list", etc...). This is
	// not interesting for our purposes, so we skip it.
	if len(args) > 0 {
		log.Tracef("The go command is %q\n", args[0])
		args = args[1:]
	}

	// Compose the complete list of arguments: those from GOFLAGS, and the rest of the command line so far; in this order
	// as the CLI arguments have precedence over those from GOFLAGS.
	args = append(append(make([]string, 0, len(goflagsArgs)+len(args)), goflagsArgs...), args...)

	var positional []string
	for i := 0; i < len(args); i++ {
		arg := args[i]

		// Any argument after "--" is a positional argument, so we are done parsing.
		if arg == "--" {
			positional = args[i+1:]
			break
		}

		// Any argument without a leading "-" is a positional argument, and the go CLI demands all flags are placed before
		// positional arguments, so we are done parsing.
		if !strings.HasPrefix(arg, "-") {
			positional = args[i:]
			break
		}

		normArg := arg
		if strings.HasPrefix(arg, "--") {
			// The Go CLI accepts flags with two hyphens instead of one, but we want
			// to normalize to a single hyphen here...
			normArg = arg[1:]
		}

		if key, val, isAssigned := strings.Cut(normArg, "="); isAssigned {
			if isLong(key) {
				flags.Long[key] = val
			} else {
				// Intentionally the un-normalized variant in Unknown flags.
				flags.Unknown = append(flags.Unknown, arg)
			}
		} else if isLong(normArg) {
			flags.Long[normArg] = args[i+1]
			i++
		} else if isShort(normArg) {
			flags.Short[normArg] = struct{}{}
		} else {
			// Intentionally the un-normalized variant in Unknown flags.
			flags.Unknown = append(flags.Unknown, arg)
			// If there's more args, and the next one does not have a leading -, we'll assume this is the value of this
			// unknown flag and consume it.
			if len(args) > i+1 && !strings.HasPrefix(args[i+1], "-") {
				flags.Unknown = append(flags.Unknown, args[i+1])
				i++
			}
		}
	}

	return flags, flags.inferCoverpkg(wd, positional)
}

// inferCoverpkg will add the necessary `-coverpkg` argument if the `-cover` flags is present and `-coverpkg` is not, as
// otherwise, sub-commands triggered with these flags will not apply coverage to the intended packages.
func (f *CommandFlags) inferCoverpkg(wd string, positionalArgs []string) error {
	if _, hasCoverpkg := f.Long["-coverpkg"]; hasCoverpkg {
		return nil
	}
	if _, isCover := f.Short["-cover"]; !isCover {
		return nil
	}

	pkgs, err := packages.Load(
		&packages.Config{
			Mode:       packages.NeedName,
			Dir:        wd,
			BuildFlags: append(f.Slice(), "-toolexec"), // Make sure we satisfy the same build constraints; but don't run -toolexec
			Logf:       func(format string, args ...any) { log.Tracef(format+"\n", args...) },
		},
		positionalArgs...,
	)
	if err != nil {
		return fmt.Errorf("failed to resolve package list from %q: %w", positionalArgs, err)
	}

	coverpkg := make([]string, len(pkgs))
	for i, pkg := range pkgs {
		coverpkg[i] = pkg.PkgPath
	}
	val := strings.Join(coverpkg, ",")
	log.Tracef("Inferred -coverpkg=%q from %q\n", val, positionalArgs)
	f.Long["-coverpkg"] = val

	return nil
}

// Flags return the top level go command flags
func Flags() (CommandFlags, error) {
	once.Do(func() {
		flags, flagsErr = parentGoCommandFlags()
	})
	return flags, flagsErr
}

// SetFlags sets the flags for this process to those parsed from the provided
// slice. Does nothing if SetFlags or Flags has already been called once.
func SetFlags(wd string, args []string) {
	once.Do(func() {
		flags, flagsErr = ParseCommandFlags(wd, args)
	})
}

func isLong(str string) bool {
	_, ok := longFlags[str]
	return ok
}

func isShort(str string) bool {
	_, ok := shortFlags[str]
	return ok
}

// parentGoCommandFlags backtracks through the process tree
// to find a parent go command invocation and returns its arguments
func parentGoCommandFlags() (flags CommandFlags, err error) {
	goBin, err := goenv.GoBinPath()
	if err != nil {
		return flags, fmt.Errorf("failed to resolve go command path: %w", err)
	}

	p, err := process.NewProcess(int32(os.Getpid()))
	if err != nil {
		return flags, fmt.Errorf("failed to get handle of the current process: %w", err)
	}

	// Backtrack through the process stack until we find the parent Go command
	var args []string
	for {
		p, err = p.Parent()
		if err != nil {
			return flags, fmt.Errorf("failed to find parent process of %d: %w", p.Pid, err)
		}
		args, err = p.CmdlineSlice()
		if err != nil {
			return flags, fmt.Errorf("failed to get command line of %d: %w", p.Pid, err)
		}
		cmd, err := exec.LookPath(args[0])
		if err != nil {
			return flags, fmt.Errorf("failed to resolve argv0 (%q) of %d: %w", args[0], p.Pid, err)
		}
		// Found the go command process, break out of backtracking
		if cmd == goBin {
			break
		}
	}

	wd, err := p.Cwd()
	if err != nil {
		return flags, fmt.Errorf("failed to get working directory of %d: %w", p.Pid, err)
	}

	return ParseCommandFlags(wd, args[2:])
}

var (
	flags    CommandFlags
	flagsErr error
	once     sync.Once
)
