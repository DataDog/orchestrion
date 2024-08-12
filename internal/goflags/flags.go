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
	"path/filepath"
	"strings"
	"sync"

	"github.com/datadog/orchestrion/internal/goenv"
	"github.com/datadog/orchestrion/internal/log"
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
// and returns its flags. Direct arguments to the command are ignored
func ParseCommandFlags(wd string, args []string) CommandFlags {
	flags := CommandFlags{
		Long:  make(map[string]string, len(args)),
		Short: make(map[string]struct{}, len(args)),
	}
	if len(args) == 0 {
		return flags
	}

	if arg := args[0]; strings.HasPrefix(arg, "-C") {
		// The first argument is a change directory flag, which we'll ignore...
		var cdir string
		if arg == "-C" && len(args) > 1 {
			// In this case, the value of `-C` is the next argument, so skip both.
			cdir = args[1]
			args = args[2:]
			log.Tracef("Skipping -C flag arguments %q %q\n", arg, cdir)
		} else {
			log.Tracef("Skipping -C flag argument %q\n", arg)
			cdir = arg[3:] // Skip over `-C=`
			args = args[1:]
		}
		if !filepath.IsAbs(cdir) {
			cdir = filepath.Join(wd, cdir)
		}
		wd = cdir
	}

	// The next argument immediately after a possible `-C` flags is the go command itself, which we are not interested in.
	log.Tracef("The go command is %q\n", args[0])

	var positional []string
	for i := 1; i < len(args); i += 1 {
		arg := args[i]
		if arg == "--" {
			// Everything after "--" is positional arguments...
			positional = args[i+1:]
			break
		}
		if !strings.HasPrefix(arg, "-") {
			// No leading - means this is actually a positional argument, and go CLI
			// requires all flags are provided before positional arguments, so
			// everything from now on is a positional argument.
			positional = args[i:]
			break
		}

		normArg := arg
		if strings.HasPrefix(arg, "--") {
			// The Go CLI accepts flags with two hyphens instead of one, but we want
			// to normalize to a single hyphen here...
			normArg = arg[1:]
		}

		if isAssigned(normArg) {
			key, val, found := strings.Cut(normArg, "=")
			if found {
				flags.Long[key] = val
			}
		} else if isLong(normArg) {
			flags.Long[normArg] = args[i+1]
			i++
		} else if isShort(normArg) {
			flags.Short[normArg] = struct{}{}
		} else {
			// We intentionally keep the original arg value in this case instead of the normalized one.
			flags.Unknown = append(flags.Unknown, arg)
		}
	}

	if _, hasCover := flags.Short["-cover"]; !hasCover {
		return flags
	}
	if _, hasCoverPkg := flags.Long["-coverpkg"]; hasCoverPkg {
		return flags
	}
	// At this point, we have `-cover` but not `-coverpkg`, so we need to infer the correct
	// `-coverpkg` argument to imitate the default behavior, for otherwise our attempts at resolving
	// dependencies might not uniformly apply `-cover` (and `-covermode` if set), which would lead to
	// link-time fingerprint mismatches.
	pkgs, err := packages.Load(&packages.Config{Mode: packages.NeedName, Dir: wd, Logf: func(format string, args ...any) { log.Tracef(format, args...) }}, positional...)
	if err != nil {
		log.Warnf("Failed to infer -coverpkg argument from positional arguments %q: %v\n", positional, err)
		return flags
	}
	coverpkg := make([]string, len(pkgs))
	for i, pkg := range pkgs {
		coverpkg[i] = pkg.PkgPath
	}
	val := strings.Join(coverpkg, " ")
	flags.Long["-coverpkg"] = val
	log.Debugf("Inferred -coverpkg=%q from positional arguments %q\n", val, positional)

	return flags
}

// Flags return the top level go command flags.
func Flags() (CommandFlags, error) {
	once.Do(func() {
		flags, flagsErr = parentGoCommandFlags()
	})
	return flags, flagsErr
}

// SetFlags sets the top level go command flags to the specified value. Does nothing if the flags are already set,
// either because `SetFlags` has already been called, or because `Flags` has been called and the flags have been set by
// it.
func SetFlags(wd string, args []string) {
	once.Do(func() {
		flags = ParseCommandFlags(wd, args)
	})
}

func isAssigned(str string) bool {
	if !strings.HasPrefix(str, "-") {
		return false
	}
	flag, _, ok := strings.Cut(str, "=")
	// An assigned flag is a long flag using the '=' separator
	return ok && isLong(flag)
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
		return flags, fmt.Errorf("failed to get process handle:%w", err)
	}

	// Backtrack through the process stack until we find the parent Go command
	var args []string
	for {
		p, err = p.Parent()
		if err != nil {
			return flags, fmt.Errorf("failed to access parent process: %w", err)
		}
		args, err = p.CmdlineSlice()
		if err != nil {
			return flags, fmt.Errorf("failed to read process %d command line: %w", p.Pid, err)
		}
		cmd, err := exec.LookPath(args[0])
		if err != nil {
			return flags, fmt.Errorf("failed to resolve %q: %w", args[0], err)
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

	return ParseCommandFlags(wd, args[1:]), nil
}

var (
	flags    CommandFlags
	flagsErr error
	once     sync.Once
)
