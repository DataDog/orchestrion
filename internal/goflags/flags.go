// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

// Package goflags allows parsing go command invocations and storing their flags in a
// CommandFlags structure. It also provides utilities to backtrack through the process stack to
// find and parse the flags of the first parent go command found in the process hierarchy.
package goflags

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"

	"github.com/DataDog/orchestrion/internal/goenv"
	"github.com/DataDog/orchestrion/internal/goflags/quoted"
	"github.com/rs/zerolog"
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
		"-a":          {}, // Rebuild everything, ignoring cached artifacts
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
		"-toolexec":   {}, // Set the command to run around tool execution
	}
)

// Get returns the value of the specified long-form flag if present. The name is
// provided including the leading hyphen, e.g: "-tags".
func (f CommandFlags) Get(flag string) (val string, found bool) {
	val, found = f.Long[flag]
	return
}

// Except returns a copy of this CommandFlags with the specified flags removed.
// The [CommandFlags.Unknown] field is not modified, even if it is in the list
// of flags to be removed.
func (f CommandFlags) Except(remove ...string) CommandFlags {
	res := CommandFlags{Unknown: f.Unknown}

	res.Short = make(map[string]struct{}, len(f.Short))
	for k, v := range f.Short {
		if slices.Contains(remove, k) {
			continue
		}
		res.Short[k] = v
	}

	res.Long = make(map[string]string, len(f.Long))
	for k, v := range f.Long {
		if slices.Contains(remove, k) {
			continue
		}
		res.Long[k] = v
	}

	return res
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
func ParseCommandFlags(ctx context.Context, wd string, args []string) (CommandFlags, error) {
	log := zerolog.Ctx(ctx)

	flags := CommandFlags{
		Long:  make(map[string]string, len(longFlags)),
		Short: make(map[string]struct{}, len(shortFlags)),
	}

	goflags := os.Getenv("GOFLAGS")
	goflagsArgs, err := quoted.Split(goflags)
	if err != nil {
		log.Warn().Str("GOFLAGS", goflags).Err(err).Msg("Failed to interpret quoted strings in GOFLAGS")
	} else {
		log.Trace().Strs("GOFLAGS", goflagsArgs).Msg("GOFLAGS arguments")
	}

	// Remove any `-C` flag provided on the command line. This is required to immediately follow the `go` command, and
	// can be present only once.
	if len(args) > 0 {
		if arg := args[0]; strings.HasPrefix(arg, "-C") {
			if arg == "-C" && len(args) > 1 {
				// ["-C", "directory", ...]
				log.Trace().Strs("flag", args[:2]).Msg("Skipping '-C <dir>' flag")
				args = args[2:]
			} else if arg[:3] == "-C=" {
				// ["-C=directory", ...]
				log.Trace().Str("flag", arg).Msg("Skipping '-C=<dir>' flag")
				args = args[1:]
			}
		}
	}

	// The next argument after a `-C` (if present) would be the go command name ("run", "test", "list", etc...). This is
	// not interesting for our purposes, so we skip it.
	if len(args) > 0 {
		log.Trace().Str("command", args[0]).Msg("Go command from arguments")
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

	if err := flags.inferCoverpkg(ctx, wd, positional); err != nil {
		return flags, err
	}

	log.Trace().Any("flags", flags).Msg("Parsed flags")
	return flags, nil
}

// inferCoverpkg will add the necessary `-coverpkg` argument if the `-cover` flags is present and
// `-coverpkg` is not, as otherwise, sub-commands triggered with these flags will not apply coverage
// to the intended packages.
// If `-coverpkg` is present, it will expand any relative paths (recognized by a `./` prefix) into
// absolute package names, so that child builds do not interpret these relative to a different
// package root.
func (f *CommandFlags) inferCoverpkg(ctx context.Context, wd string, positionalArgs []string) error {
	log := zerolog.Ctx(ctx)

	// Make sure we satisfy the same build constraints; but don't run -toolexec
	childBuildFlags := append(f.Slice(), "-toolexec=")
	childBuildLogf := func(format string, args ...any) {
		log.Trace().Str("operation", "packages.Load").Msgf(format, args...)
	}

	if val, hasCoverpkg := f.Long["-coverpkg"]; hasCoverpkg {
		if val == "" {
			// Blank specified, not trying to expand it...
			return nil
		}

		// We have patterns, we need to make sure they are expressed in absolute terms.
		var newValBuf strings.Builder
		newValBuf.Grow(len(val))

		for idx, pattern := range strings.Split(val, ",") {
			if idx > 0 {
				_ = newValBuf.WriteByte(',')
			}
			if !strings.HasPrefix(pattern, "./") && !strings.HasPrefix(pattern, ".\\") {
				// If the pattern is not relative, so we're good.
				_, _ = newValBuf.WriteString(pattern)
				continue
			}

			log.Debug().
				Str("-coverpkg.entry", pattern).
				Msg("Resolving relative -coverpkg entry")
			pkgs, err := packages.Load(&packages.Config{
				Mode:       packages.NeedName,
				Dir:        wd,
				BuildFlags: childBuildFlags,
				Logf:       childBuildLogf,
			}, pattern)
			if err != nil {
				return fmt.Errorf("resolving -coverpkg entry %q: %w", pattern, err)
			}
			for idx, pkg := range pkgs {
				if len(pkg.Errors) != 0 {
					var err error
					for _, pkgErr := range pkg.Errors {
						err = errors.Join(err, pkgErr)
					}
					log.Warn().
						Err(err).
						Str("pkg.ID", pkg.ID).
						Str("-coverpkg.entry", pattern).
						Msg("Error when resolving -coverpkg entry")
				}

				if idx > 0 {
					_ = newValBuf.WriteByte(',')
				}
				_, _ = newValBuf.WriteString(pkg.PkgPath)
			}
		}

		newVal := newValBuf.String()
		f.Long["-coverpkg"] = newVal
		log.Debug().
			Str("-coverpkg", newVal).
			Msg("Finalized -coverpkg value")
		return nil
	}

	_, isCover := f.Short["-cover"]
	if !isCover {
		// -covermode implies -cover
		_, isCover = f.Long["-covermode"]
	}
	if !isCover {
		return nil
	}

	pkgs, err := packages.Load(
		&packages.Config{
			Mode:       packages.NeedName,
			Dir:        wd,
			BuildFlags: childBuildFlags,
			Logf:       childBuildLogf,
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
	log.Trace().Str("-coverpkg", val).Strs("positional", positionalArgs).Msg("Inferred -coverpkg flag from positional arguments")
	f.Long["-coverpkg"] = val

	return nil
}

// Flags return the top level go command flags
func Flags(ctx context.Context) (CommandFlags, error) {
	once.Do(func() {
		flags, flagsErr = parentGoCommandFlags(ctx, os.Getpid())
	})
	return flags, flagsErr
}

// SetFlagsFromPid sets the top level go command flags by looking up the process
// tree from the specified PID. This is used by the job server when it is
// started as a daemon (and hence cannot crawl it's own process tree to find
// this information).
func SetFlagsFromPid(ctx context.Context, pid int) error {
	once.Do(func() {
		log := zerolog.Ctx(ctx)
		log.Trace().Int("process.pid", pid).Msg("Looking up parent go command flags from user-provided PID")
		flags, flagsErr = parentGoCommandFlags(ctx, pid)
	})
	return flagsErr
}

// SetFlags sets the flags for this process to those parsed from the provided
// slice. Does nothing if SetFlags or Flags has already been called once.
func SetFlags(ctx context.Context, wd string, args []string) {
	once.Do(func() {
		log := zerolog.Ctx(ctx)
		log.Trace().Strs("flags", args).Msg("Storing provided go flags")
		flags, flagsErr = ParseCommandFlags(ctx, wd, args)
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
func parentGoCommandFlags(ctx context.Context, pid int) (flags CommandFlags, err error) {
	log := zerolog.Ctx(ctx)
	log.Trace().Msg("Attempting to parse parent Go command arguments")

	goBin, err := goenv.GoBinPath()
	if err != nil {
		return flags, fmt.Errorf("failed to resolve go command path: %w", err)
	}
	log.Trace().Str("go.bin", goBin).Msg("Resolved go command path")

	p, err := process.NewProcess(int32(pid))
	if err != nil {
		return flags, fmt.Errorf("failed to get handle of the process with pid %d: %w", pid, err)
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
		// When running in containers using on macOS VZ+rosetta, the reported command line may be led by
		// the registered rosetta binfmt handler. In such cases, the argv0 has a leaf name of "rosetta"
		// and is not present within the container itself (it's only on the hypervisor). In such cases,
		// we try to resolve argv[1] instead. This can only manifest itself on amd64 + linux.
		if errors.Is(err, fs.ErrNotExist) && runtime.GOARCH == "amd64" && runtime.GOOS == "linux" && filepath.Base(args[0]) == "rosetta" && len(args) > 1 {
			log.Trace().Err(err).Msg("Attempting to resolve rosetta target after error resolving argv0")
			var err2 error
			cmd, err2 = exec.LookPath(args[1])
			if err2 != nil {
				err = errors.Join(err, fmt.Errorf("failed to resolve argv1 (%q) of %d (attempting Apple rosetta fallback): %w", args[1], p.Pid, err2))
			} else {
				// The fallback was successful, we no longer have an error!
				err = nil
				log.Trace().Str("command", cmd).Msg("Rosetta fall-back was successful")
			}
		}
		if err != nil {
			return flags, fmt.Errorf("failed to resolve argv0 (%q) of %d: %w", args[0], p.Pid, err)
		}
		// Found the go command process, break out of backtracking
		if cmd == goBin {
			break
		}

		log.Trace().Int32("process.pid", p.Pid).Strs("args", args).Msg("Not a go command process, continuing backtracking")
	}

	log.Trace().Int32("go.pid", p.Pid).Strs("arguments", args).Msg("Found parent go command process")
	wd, err := p.Cwd()
	if err != nil {
		return flags, fmt.Errorf("failed to get working directory of %d: %w", p.Pid, err)
	}

	return ParseCommandFlags(ctx, wd, args[1:])
}

var (
	flags    CommandFlags
	flagsErr error
	once     sync.Once
)
