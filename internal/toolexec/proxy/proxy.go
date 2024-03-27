// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package proxy

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

type (
	// CommandType represents a Go toolchain command type, such
	// as "compile", "link", etc...
	CommandType int
	// Command represents a Go compilation command
	Command interface {
		// Args are all the command arguments, starting from the Go tool command
		Args() []string
		ReplaceParam(param string, val string) error
		// Stage returns the build stage of the command. Each stage usually associated
		// to a specific package and is named using the `bXXX` format, where `X` are numbers.
		// Stage b001 is the final stage of the go build process
		Stage() string
		// Type represents the go tool command type (compile, link, asm, etc.)
		Type() CommandType

		// ShowVersion returns true if the command received the `-V=full` argument, signaling it should
		// print its full version information and exit. This feature is used by the go toolchain to
		// create build cache keys, and allows invalidating all build cache when the tooling changes.
		ShowVersion() bool
	}

	// CommandProcessor is a function that takes a command as input
	// and is allowed to modify it or read its data
	CommandProcessor[T Command] func(T)

	commandFlagSet struct {
		Output  string `ddflag:"-o"`
		Version string `ddflag:"-V"`
	}

	// command is the default unknown command type
	// Can be used to compose specific Command implementations
	command struct {
		args []string
		// paramPos is the index in args of the *value* provided for the parameter stored in the key
		paramPos map[string]int
		flags    commandFlagSet
	}
)

const (
	CommandTypeOther CommandType = iota
	CommandTypeCompile
	CommandTypeLink
)

// ProcessCommand applies a processor on a command if said command matches
// the input type of said input processor. Failure to match types is not
// considered to be an error.
func ProcessCommand[T Command](cmd Command, p CommandProcessor[T]) {
	if c, ok := cmd.(T); ok {
		p(c)
	}
}

// NewCommand initializes a new command object and takes care of tracking the indexes of its
// arguments
func NewCommand(args []string) command {
	cmd := command{
		args:     args,
		paramPos: make(map[string]int),
	}
	for pos, v := range args[1:] {
		cmd.paramPos[v] = pos + 1
	}

	parseFlags(&cmd.flags, args)

	return cmd
}

func (cmd *command) ShowVersion() bool {
	return cmd.flags.Version == "full"
}

// ReplaceParam will replace any parameter of the command provided it is found
// A parameter can be a flag, an option, a value, etc
func (cmd *command) ReplaceParam(param string, val string) error {
	i, ok := cmd.paramPos[param]
	if !ok {
		return fmt.Errorf("%s not found", param)
	}
	cmd.args[i] = val
	delete(cmd.paramPos, param)
	cmd.paramPos[val] = i
	return nil
}

// RunCommandOption allows customizing a run command before execution. For example, this can be used
// to capture the output of the command instead of forwarding it to the host process' STDIO.
type RunCommandOption func(*exec.Cmd)

// RunCommand executes the underlying go tool command and forwards the program's standard fluxes
func RunCommand(cmd Command, opts ...RunCommandOption) error {
	args := cmd.Args()
	c := exec.Command(args[0], args[1:]...)
	if c == nil {
		return fmt.Errorf("command couldn't build")
	}

	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr

	for _, opt := range opts {
		opt(c)
	}

	return c.Run()
}

// MustRunCommand is like RunCommand but panics if the command fails to build or run
func MustRunCommand(cmd Command, opts ...RunCommandOption) {
	var exitErr *exec.ExitError
	err := RunCommand(cmd, opts...)
	if err == nil {
		return
	}
	if errors.As(err, &exitErr) {
		os.Exit(exitErr.ExitCode())
	}
	panic(err)
}

func (cmd *command) Stage() string {
	return filepath.Base(filepath.Dir(cmd.flags.Output))
}

func (cmd *command) Type() CommandType {
	return CommandTypeOther
}

func (cmd *command) Args() []string {
	return cmd.args
}
