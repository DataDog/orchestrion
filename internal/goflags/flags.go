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
	"go/build"
	"go/version"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/DataDog/orchestrion/internal/goenv"
	"github.com/DataDog/orchestrion/internal/goflags/quoted"
	"github.com/rs/zerolog"
	"github.com/shirou/gopsutil/v4/process"
	"golang.org/x/tools/go/packages"
)

// CommandFlags represents the flags provided to a go command invocation
type CommandFlags struct {
	Long    map[string]string
	Short   map[string]struct{}
	Unknown []string // flags we don't process but store anyway

	// TestCoverpkgInferred reports whether -coverpkg was inferred from a go test
	// command's package arguments, rather than provided explicitly by the user.
	TestCoverpkgInferred bool
	// TestPackagesWithoutTests lists command-line packages that go test covers in
	// their ordinary form because they contain no test files.
	TestPackagesWithoutTests []string
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
		"-asmflags":      {}, // Flags passed through to the assembly
		"-buildmode":     {}, // Set build mode
		"-buildvcs":      {}, // Whether to stamp binaries with version control information
		"-compiler":      {}, // Select what compiler to use
		"-covermode":     {}, // Set coverage mode
		"-coverpkg":      {}, // Set list of packages to collect coverage for
		"-gccgoflags":    {}, // Flags passed through to the gccgo compiler
		"-gcflags":       {}, // Flags passed through to the gc compiler
		"-installsuffix": {}, // Suffix used in the package installation directory
		"-ldflags":       {}, // Flags passed through to the linker
		"-mod":           {}, // Set module download mode
		"-modfile":       {}, // Set module file
		"-overlay":       {}, // Set overlay file
		"-pgo":           {}, // Set profile-guided optimization profile file
		"-pkgdir":        {}, // Set package install & load directory
		"-tags":          {}, // Set build tags
		"-toolexec":      {}, // Set the command to run around tool execution
	}
	// coverImplyingFlags are `go test`-only flags which are not build flags (and
	// hence must not be forwarded to `go list` or child build commands, which do
	// not accept them), but which implicitly enable coverage instrumentation,
	// exactly as the `-cover` flag does. Child builds must apply the same coverage
	// instrumentation as the parent command, as otherwise archives produced by
	// child builds are not compatible with those produced by the parent command
	// (the Go linker rejects them with a "fingerprint mismatch" error).
	coverImplyingFlags = map[string]struct{}{
		"-coverprofile": {}, // Write a coverage profile to the designated file
	}
	// optionalValueFlags are flags from [longFlags] which accept an optional value,
	// meaning their bare form does not consume the argument that follows them (the
	// value can only be provided using the `-flag=value` form).
	optionalValueFlags = map[string]struct{}{
		"-buildvcs": {}, // Whether to stamp binaries with version control information
	}
	// valuelessFlags are flags Orchestrion does not otherwise process (they are
	// not build flags, and are consequently not forwarded to child commands), but
	// which are known not to accept a value. Identifying those is necessary to
	// correctly tell flags apart from positional arguments (package patterns), as
	// a flag that accepts a value consumes the argument that follows it when it is
	// provided in the `-flag value` form.
	valuelessFlags = map[string]struct{}{
		// `go test` flags
		"-artifacts": {}, // Retain test artifacts (Go 1.26 and later)
		"-benchmem":  {}, // Print memory allocation statistics for benchmarks
		"-c":         {}, // Compile the test binary but do not run it
		"-failfast":  {}, // Do not start new tests after the first test failure
		"-fullpath":  {}, // Show full file names in the error messages
		"-json":      {}, // Convert test output to JSON
		"-short":     {}, // Tell long-running tests to shorten their run time
		"-v":         {}, // Verbose output
		// Build flags that are irrelevant to (and must not be forwarded to) child builds
		"-n": {}, // Print the commands but do not run them
		"-x": {}, // Print the commands
	}
	// testValueFlags are flags recognized by `go test` which require a value and
	// which Orchestrion does not otherwise process. Knowing they are recognized is
	// necessary because package patterns may follow them, unlike flags destined to
	// the test binary. Both their bare and `-flag=value` forms are accepted.
	testValueFlags = map[string]struct{}{
		"-bench":                {},
		"-benchtime":            {},
		"-blockprofile":         {},
		"-blockprofilerate":     {},
		"-count":                {},
		"-cpu":                  {},
		"-cpuprofile":           {},
		"-debug-actiongraph":    {},
		"-debug-runtime-trace":  {},
		"-debug-trace":          {},
		"-exec":                 {},
		"-fuzz":                 {},
		"-fuzzminimizetime":     {},
		"-fuzztime":             {},
		"-list":                 {},
		"-memprofile":           {},
		"-memprofilerate":       {},
		"-mutexprofile":         {},
		"-mutexprofilefraction": {},
		"-o":                    {},
		"-outputdir":            {},
		"-p":                    {},
		"-parallel":             {},
		"-run":                  {},
		"-shuffle":              {},
		"-skip":                 {},
		"-timeout":              {},
		"-trace":                {},
		"-vet":                  {},
	}
	// versionedTestFlags records when test flags unavailable in the minimum
	// supported Go version were introduced.
	versionedTestFlags = map[string]string{
		"-artifacts": "go1.26",
	}
	// nonForwardedTestFlags are test command and build flags included in the
	// valueless and value-taking sets above for parsing purposes, but for which
	// cmd/go does not accept a `-test.`-prefixed alias.
	nonForwardedTestFlags = map[string]struct{}{
		"-c":                   {},
		"-debug-actiongraph":   {},
		"-debug-runtime-trace": {},
		"-debug-trace":         {},
		"-exec":                {},
		"-json":                {},
		"-n":                   {},
		"-o":                   {},
		"-p":                   {},
		"-vet":                 {},
		"-x":                   {},
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
	res := CommandFlags{
		Unknown:                  f.Unknown,
		TestCoverpkgInferred:     f.TestCoverpkgInferred,
		TestPackagesWithoutTests: slices.Clone(f.TestPackagesWithoutTests),
	}

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
// - unknown flags and values are ignored
func (f CommandFlags) Slice() []string {
	flags := make([]string, 0, len(f.Long)+len(f.Short))
	for flag, val := range f.Long {
		if flag == "-cover" {
			continue
		}
		flags = append(flags, fmt.Sprintf("%s=%s", flag, val))
	}
	// Coverage options imply -cover when parsed, so an explicit false override
	// must follow every assigned coverage option regardless of map iteration order.
	if val, found := f.Long["-cover"]; found {
		flags = append(flags, "-cover="+val)
	}
	for flag := range f.Short {
		flags = append(flags, flag)
	}
	return flags
}

func (f *CommandFlags) setLong(flag string, value string) {
	f.Long[flag] = value
	delete(f.Short, flag)
}

func (f *CommandFlags) setShort(flag string) {
	f.Short[flag] = struct{}{}
	delete(f.Long, flag)
}

func (f *CommandFlags) setBoolean(flag string, value string) (bool, error) {
	enabled, err := strconv.ParseBool(value)
	if err != nil {
		// Preserve the invalid value so cmd/go can report it canonically.
		f.setLong(flag, value)
		return false, err
	}
	if enabled {
		f.setShort(flag)
	} else {
		f.setLong(flag, "false")
	}
	return enabled, nil
}

func (f *CommandFlags) enableCoverage() {
	f.setShort("-cover")
}

func (f *CommandFlags) setAssignedCover(value string) {
	_, _ = f.setBoolean("-cover", value)
}

// ParseCommandFlags parses a slice representing a go command invocation
// and returns its flags. Direct arguments to the command are ignored. The value
// of $GOFLAGS is also included in the returned flags. A -C before or immediately
// after the Go command path is applied relative to wd; a blank or relative wd is
// resolved against the current process working directory.
func ParseCommandFlags(ctx context.Context, wd string, args []string) (CommandFlags, error) {
	log := zerolog.Ctx(ctx)
	effectiveWD, args, changed, err := resolveLeadingChdir(wd, args)
	if err != nil {
		return CommandFlags{}, fmt.Errorf("resolving leading -C flag: %w", err)
	}
	if changed {
		log.Trace().Str("from", wd).Str("to", effectiveWD).Msg("Applying leading -C flag")
	}

	goVersion, err := goenv.GOVERSION(effectiveWD)
	if err != nil {
		return CommandFlags{}, fmt.Errorf("determining Go toolchain version: %w", err)
	}
	return parseCommandFlags(ctx, effectiveWD, args, goVersion)
}

// resolveLeadingChdir applies the leading -C flag handled specially by cmd/go
// and removes it from args. The returned working directory is absolute when the
// flag designates a relative directory.
func resolveLeadingChdir(wd string, args []string) (effectiveWD string, remaining []string, changed bool, err error) {
	dir, remaining, changed := leadingChdirFlag(args)
	if !changed {
		return wd, args, false, nil
	}
	if filepath.IsAbs(dir) {
		return filepath.Clean(dir), remaining, true, nil
	}
	// Rooted and drive-relative paths on Windows are relative to process state,
	// not to an arbitrary wd. filepath.Abs reproduces os.Chdir's resolution.
	if filepath.VolumeName(dir) != "" || len(dir) > 0 && os.IsPathSeparator(dir[0]) {
		effectiveWD, err = filepath.Abs(dir)
	} else {
		effectiveWD = filepath.Join(wd, dir)
		if !filepath.IsAbs(effectiveWD) {
			effectiveWD, err = filepath.Abs(effectiveWD)
		}
	}
	if err != nil {
		return "", nil, false, err
	}
	return filepath.Clean(effectiveWD), remaining, true, nil
}

// leadingChdirFlag returns the directory and remaining arguments when args
// contains one of the leading -C forms accepted by cmd/go. The flag may precede
// the command or immediately follow the command (or command-group) token.
func leadingChdirFlag(args []string) (dir string, remaining []string, found bool) {
	indices := [...]int{0, 1, 2}
	for _, index := range indices {
		if index >= len(args) || index == 1 && strings.HasPrefix(args[0], "-") {
			continue
		}
		if index == 2 && (args[0] != "mod" && args[0] != "work" || strings.HasPrefix(args[1], "-")) {
			continue
		}

		consumed := 0
		switch arg := args[index]; {
		case arg == "-C" || arg == "--C":
			if index+1 >= len(args) {
				continue
			}
			dir = args[index+1]
			consumed = 2
		case strings.HasPrefix(arg, "-C=") || strings.HasPrefix(arg, "--C="):
			_, dir, _ = strings.Cut(arg, "=")
			consumed = 1
		default:
			continue
		}

		remaining = make([]string, 0, len(args)-consumed)
		remaining = append(remaining, args[:index]...)
		remaining = append(remaining, args[index+consumed:]...)
		return dir, remaining, true
	}
	return "", args, false
}

// runTarget returns the package portion of the arguments remaining after go
// run's flag parser stops. A target is one package pattern or a contiguous list
// of .go files; every subsequent argument is passed to the program.
func runTarget(args []string) []string {
	if len(args) == 0 || !strings.HasSuffix(args[0], ".go") {
		return args[:min(len(args), 1)]
	}
	for i, arg := range args {
		if !strings.HasSuffix(arg, ".go") {
			return args[:i]
		}
	}
	return args
}

func parseCommandFlags(ctx context.Context, wd string, args []string, goVersion string) (CommandFlags, error) {
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

	// The first argument is the go command name ("run", "test", "list", etc...).
	var command string
	if len(args) > 0 {
		command = args[0]
		log.Trace().Str("command", command).Msg("Go command from arguments")
		args = args[1:]
	}
	// Some arguments are only meaningful to `go test`. The Go CLI silently ignores flags from $GOFLAGS
	// that the command at hand does not accept, and fails on unaccepted flags from the command line; so
	// we must not honor test-only flags when running another command.
	isTestCommand := command == "test"

	// Compose the complete list of arguments: those from GOFLAGS, and the rest of the command line so far; in this order
	// as the CLI arguments have precedence over those from GOFLAGS.
	goflagsCount := len(goflagsArgs)
	args = append(append(make([]string, 0, len(goflagsArgs)+len(args)), goflagsArgs...), args...)

	var (
		positional []string
		// The Go CLI accepts package patterns as a single contiguous run of non-flag arguments: any
		// non-flag argument that follows a flag once that run has ended is a flag value or an argument
		// destined to the built program (see `cmd/go/internal/test.testFlags`), and must consequently not
		// be mistaken for a package pattern.
		positionalRunEnded bool
	)
	for i := 0; i < len(args); i++ {
		arg := args[i]

		// Any argument after "--" is a positional argument, so we are done parsing. If package patterns
		// were already provided, the arguments that follow are destined to the built program instead.
		// The `go test` command is special, as it never accepts package patterns after the "--" marker.
		if arg == "--" {
			if !isTestCommand && len(positional) == 0 {
				rest := args[i+1:]
				if command == "run" {
					rest = runTarget(rest)
				}
				positional = append(positional, rest...)
			}
			break
		}

		// Every argument after "-args" is passed to the built test binary, so we are done parsing.
		if isTestCommand && (arg == "-args" || arg == "--args") {
			break
		}

		// Any argument without a leading "-" is a positional argument (until proven otherwise).
		if !strings.HasPrefix(arg, "-") {
			if positionalRunEnded {
				// This cannot be a package pattern. It is a literal argument to the test binary, and
				// cmd/go consequently passes the entire remainder through without parsing more flags.
				break
			}
			if command == "run" {
				// `go run` accepts one package pattern or a contiguous list of .go files;
				// everything after that package is passed to the program.
				positional = append(positional, runTarget(args[i:])...)
				break
			}
			positional = append(positional, arg)
			continue
		}
		// Except for go test's custom parser, standard flag parsing stops at the
		// first positional argument and leaves every subsequent token untouched.
		if !isTestCommand && len(positional) > 0 {
			break
		}
		// This argument is a flag, so any positional argument run has now ended.
		if len(positional) > 0 {
			positionalRunEnded = true
		}

		normArg := arg
		if strings.HasPrefix(arg, "--") {
			// The Go CLI accepts flags with two hyphens instead of one, but we want
			// to normalize to a single hyphen here...
			normArg = arg[1:]
		}

		key, val, isAssigned := strings.Cut(normArg, "=")
		hasNextArg := i+1 < len(args)
		fromGOFLAGS := i < goflagsCount
		if fromGOFLAGS && isBuildCoverageFlag(key) && !commandSupportsCoverage(command) {
			// cmd/go accepts coverage flags in GOFLAGS because some commands support them,
			// but silently ignores them for commands without coverage build support.
			flags.Unknown = append(flags.Unknown, arg)
			continue
		}
		switch {
		case isAssigned && isLong(key):
			flags.setLong(key, val)
			if key == "-covermode" || key == "-coverpkg" {
				flags.enableCoverage()
			}

		case isAssigned && isShort(key):
			if key == "-cover" {
				flags.setAssignedCover(val)
			} else {
				_, _ = flags.setBoolean(key, val)
			}

		case isAssigned:
			if isTestCommand {
				flags.honorTestOnlyFlag(log, key)
				if !fromGOFLAGS && !isKnownTestFlag(key, goVersion) {
					// cmd/go treats an unknown command-line flag as the end of the package list,
					// even when its value is assigned inline. Flags from GOFLAGS that are known
					// to another go command are ignored instead.
					positionalRunEnded = true
				}
			}
			// Intentionally the un-normalized variant in Unknown flags.
			flags.Unknown = append(flags.Unknown, arg)

		case isOptionalValue(normArg):
			// The bare form of these flags does not consume the argument that follows it.
			flags.setShort(normArg)

		case isLong(normArg) && hasNextArg && !fromGOFLAGS:
			flags.setLong(normArg, args[i+1])
			if normArg == "-covermode" || normArg == "-coverpkg" {
				flags.enableCoverage()
			}
			i++

		case isLong(normArg):
			// The flag is missing its value; let the Go CLI report this error in its canonical way.
			log.Trace().Str("flag", arg).Msg("Ignoring flag that is missing its value")
			flags.Unknown = append(flags.Unknown, arg)

		case isShort(normArg):
			flags.setShort(normArg)

		default:
			if isTestCommand {
				flags.honorTestOnlyFlag(log, normArg)
				if !fromGOFLAGS && !isKnownTestFlag(normArg, goVersion) {
					// Unknown test-binary flags end the package list. Their possible value is
					// consumed below, after which recognized go test flags may still follow.
					positionalRunEnded = true
				}
			}
			// Intentionally the un-normalized variant in Unknown flags.
			flags.Unknown = append(flags.Unknown, arg)
			// If this flag consumes the argument that follows it, that argument is its value, and not a
			// positional argument. A GOFLAGS entry must not consume the first command-line argument:
			// cmd/go considers each GOFLAGS token independently and ignores flags from another command.
			rest := args[i+1:]
			if fromGOFLAGS && len(rest) > goflagsCount-i-1 {
				rest = rest[:goflagsCount-i-1]
			}
			if consumesValue(normArg, rest, goVersion) {
				flags.Unknown = append(flags.Unknown, args[i+1])
				i++
			}
		}
	}

	if err := flags.inferCoverpkg(ctx, wd, command, positional); err != nil {
		return flags, err
	}

	log.Trace().Any("flags", flags).Msg("Parsed flags")
	return flags, nil
}

// inferCoverpkg will add the necessary `-coverpkg` argument if the `-cover` flags is present and
// `-coverpkg` is not, as otherwise, sub-commands triggered with these flags will not apply coverage
// to the intended packages.
// Coverage may have been enabled by way of `-cover`, `-covermode`, or one of the [coverImplyingFlags]
// (in which case `-cover` was added to the parsed flags during parsing).
// If `-coverpkg` is present, it will expand any relative paths (recognized by a `./` prefix) into
// absolute package names, so that child builds do not interpret these relative to a different
// package root.
func (f *CommandFlags) inferCoverpkg(ctx context.Context, wd string, command string, positionalArgs []string) error {
	log := zerolog.Ctx(ctx)

	if val, assigned := f.Long["-cover"]; assigned {
		if enabled, err := strconv.ParseBool(val); err == nil && !enabled {
			// A trailing explicit false overrides every earlier coverage-implying flag.
			return nil
		}
	}

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
	if command == "run" && usesOutsideModuleMode(positionalArgs) {
		// cmd/go resolves path@version run targets outside the current module and
		// leaves -coverpkg unset. packages.Load cannot resolve this syntax.
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
	if command == "test" {
		f.TestCoverpkgInferred = true
		f.TestPackagesWithoutTests, err = packagesWithoutTests(&packages.Config{
			Context:    ctx,
			Mode:       packages.NeedName | packages.NeedForTest,
			Dir:        wd,
			BuildFlags: childBuildFlags,
			Tests:      true,
			Logf:       childBuildLogf,
		}, positionalArgs, pkgs)
		if err != nil {
			return err
		}
	}

	return nil
}

func packagesWithoutTests(config *packages.Config, patterns []string, targets []*packages.Package) ([]string, error) {
	loaded, err := packages.Load(config, patterns...)
	if err != nil {
		return nil, fmt.Errorf("determining which covered packages have tests: %w", err)
	}

	withTests := make(map[string]struct{})
	for _, pkg := range loaded {
		if pkg.ForTest != "" {
			withTests[pkg.ForTest] = struct{}{}
		}
		if len(pkg.Errors) == 0 {
			continue
		}

		var pkgErr error
		for _, err := range pkg.Errors {
			pkgErr = errors.Join(pkgErr, err)
		}
		zerolog.Ctx(config.Context).Warn().Err(pkgErr).Str("pkg.ID", pkg.ID).Msg("Error while determining whether package has tests")
		// Preserve the outer go command's canonical failure instead of risking a
		// fingerprint mismatch caused by treating incomplete test metadata as proof
		// that a package has no tests.
		path := pkg.ForTest
		if path == "" {
			path = pkg.PkgPath
		}
		withTests[path] = struct{}{}
	}

	withoutTests := make([]string, 0, len(targets))
	for _, pkg := range targets {
		if _, found := withTests[pkg.PkgPath]; !found {
			withoutTests = append(withoutTests, pkg.PkgPath)
		}
	}
	if len(withoutTests) == 0 {
		return nil, nil
	}
	return withoutTests, nil
}

// usesOutsideModuleMode reports whether go run resolves its target outside the
// current module, mirroring cmd/go/internal/run.shouldUseOutsideModuleMode.
func usesOutsideModuleMode(args []string) bool {
	return len(args) > 0 &&
		!strings.HasSuffix(args[0], ".go") &&
		!strings.HasPrefix(args[0], "-") &&
		strings.Contains(args[0], "@") &&
		!build.IsLocalImport(args[0]) &&
		!filepath.IsAbs(args[0])
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
// slice. wd is the working directory before any -C adjacent to the Go command
// path; a blank or relative wd is resolved against the current process working
// directory. Does nothing if SetFlags or Flags has already been called once.
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

func isBuildCoverageFlag(flag string) bool {
	return flag == "-cover" || flag == "-covermode" || flag == "-coverpkg"
}

func commandSupportsCoverage(command string) bool {
	switch command {
	case "build", "install", "list", "run", "test":
		return true
	default:
		return false
	}
}

// impliesCover returns true if the provided flag name (including its leading
// hyphen, e.g: "-coverprofile") implicitly enables coverage instrumentation.
func impliesCover(str string) bool {
	_, ok := coverImplyingFlags[canonicalTestFlag(str)]
	return ok
}

// isValueless returns true if the provided flag name (including its leading
// hyphen, e.g: "-v") is known not to accept a value in the current Go version.
func isValueless(str string, goVersion string) bool {
	name := canonicalTestFlag(str)
	if !testFlagSupported(name, goVersion) {
		return false
	}
	_, ok := valuelessFlags[name]
	return ok
}

// isKnownTestFlag returns true if the provided flag is recognized by `go test`
// in the current Go version. Unlike unknown test-binary flags, recognized flags
// do not prevent a package list from following them.
func isKnownTestFlag(str string, goVersion string) bool {
	known, _ := testFlagTakesValue(str, goVersion)
	return known
}

// testFlagTakesValue reports whether a flag is recognized by `go test` in the
// designated Go version and, if so, whether its bare form consumes the next
// argument regardless of whether that value begins with a hyphen.
func testFlagTakesValue(str string, goVersion string) (known bool, takesValue bool) {
	name := canonicalTestFlag(str)
	if !testFlagSupported(name, goVersion) {
		return false, false
	}
	if isOptionalValue(name) || isShort(name) || isValueless(name, goVersion) {
		return true, false
	}
	if isLong(name) {
		return true, true
	}
	if _, ok := testValueFlags[name]; ok {
		return true, true
	}
	if impliesCover(name) {
		return true, true
	}
	return false, false
}

// testFlagSupported returns whether a versioned test flag is available in the
// designated Go toolchain. Development toolchains are assumed to support the
// latest known flags.
func testFlagSupported(name string, goVersion string) bool {
	minimum, versioned := versionedTestFlags[canonicalTestFlag(name)]
	if !versioned {
		return true
	}
	return !version.IsValid(goVersion) || version.Compare(goVersion, minimum) >= 0
}

// consumesValue returns true if the provided flag, which Orchestrion does not
// otherwise process, consumes the first of the arguments that follow it as its
// value.
func consumesValue(flag string, rest []string, goVersion string) bool {
	if len(rest) == 0 {
		return false
	}
	if known, takesValue := testFlagTakesValue(flag, goVersion); known {
		return takesValue
	}
	// cmd/go cannot know an unknown test-binary flag's arity. It optimistically
	// treats the next non-flag argument as its value, but parses a hyphen-prefixed
	// argument as another flag.
	return !strings.HasPrefix(rest[0], "-")
}

// honorTestOnlyFlag records the side effects of `go test`-only flags that
// Orchestrion does not otherwise process; meaning those that implicitly enable
// coverage instrumentation.
func (f *CommandFlags) honorTestOnlyFlag(log *zerolog.Logger, flag string) {
	if !impliesCover(flag) {
		return
	}
	log.Trace().Str("flag", flag).Msg("Flag implies coverage instrumentation is enabled")
	f.enableCoverage()
}

// isOptionalValue returns true if the provided flag name (including its leading
// hyphen, e.g: "-buildvcs") accepts an optional value, meaning its bare form
// does not consume the argument that follows it.
func isOptionalValue(str string) bool {
	_, ok := optionalValueFlags[str]
	return ok
}

// canonicalTestFlag removes the `test.` prefix from test flag names when cmd/go
// accepts that alias for a flag it forwards to the test binary.
func canonicalTestFlag(str string) string {
	name, isPrefixed := strings.CutPrefix(str, "-test.")
	if !isPrefixed {
		return str
	}
	name = "-" + name
	if _, nonForwarded := nonForwardedTestFlags[name]; nonForwarded {
		return str
	}
	if _, forwarded := valuelessFlags[name]; forwarded {
		return name
	}
	if _, forwarded := testValueFlags[name]; forwarded {
		return name
	}
	if _, forwarded := coverImplyingFlags[name]; forwarded {
		return name
	}
	return str
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

	p, err := process.NewProcessWithContext(ctx, int32(pid))
	if err != nil {
		return flags, fmt.Errorf("failed to get handle of the process with pid %d: %w", pid, err)
	}

	// Backtrack through the process stack until we find the parent Go command
	var args []string
	for {
		p, err = p.ParentWithContext(ctx)
		if err != nil {
			return flags, fmt.Errorf("failed to find parent process of %d: %w", p.Pid, err)
		}
		args, err = p.CmdlineSliceWithContext(ctx)
		if err != nil {
			return flags, fmt.Errorf("failed to get command line of %d: %w", p.Pid, err)
		}

		cmd, err := exec.LookPath(args[0])
		// When running in containers using on macOS VZ+rosetta, the reported command line may be led by
		// the registered rosetta binfmt handler. In such cases, the argv0 has a leaf name of "rosetta"
		// and is not present within the container itself (it's only on the hypervisor). In such cases,
		// we try to resolve argv[1] instead. This can only manifest itself on amd64 + linux.
		notExist := errors.Is(err, fs.ErrNotExist) || (err != nil && strings.Contains(err.Error(), "executable file not found"))
		base := filepath.Base(args[0])
		emulation := runtime.GOARCH == "amd64" && strings.Contains(base, "rosetta")
		emulation = emulation || strings.Contains(base, "qemu")
		if notExist && runtime.GOOS == "linux" && emulation && len(args) > 1 {
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

	commandArgs := args[1:]
	if _, remaining, found := leadingChdirFlag(commandArgs); found {
		// cmd/go changes its process working directory before doing any other work.
		// The parent process' cwd already reflects -C, so only remove the original
		// argv entry here instead of applying it a second time.
		commandArgs = remaining
	}
	return ParseCommandFlags(ctx, wd, commandArgs)
}

var (
	flags    CommandFlags
	flagsErr error
	once     sync.Once
)
