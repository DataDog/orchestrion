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

	"github.com/datadog/orchestrion/internal/goproxy"
	"github.com/shirou/gopsutil/v3/process"
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
func ParseCommandFlags(args []string) CommandFlags {
	flags := CommandFlags{
		Long:  make(map[string]string, len(args)),
		Short: make(map[string]struct{}, len(args)),
	}

	for i := 0; i < len(args); i += 1 {
		arg := args[i]
		if isAssigned(arg) {
			key, val, found := strings.Cut(arg, "=")
			if found {
				flags.Long[key] = val
			}
		} else if isLong(arg) {
			flags.Long[arg] = args[i+1]
			i++
		} else if isShort(arg) {
			flags.Short[arg] = struct{}{}
		} else {
			flags.Unknown = append(flags.Unknown, arg)
		}
	}

	return flags
}

// Flags return the top level go command flags
func Flags() (CommandFlags, error) {
	once.Do(func() {
		flags, flagsErr = parentGoCommandFlags()
	})
	return flags, flagsErr
}

func isAssigned(str string) bool {
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

	return ParseCommandFlags(args[2:]), nil
}

var (
	flags    CommandFlags
	flagsErr error
	once     sync.Once
)
