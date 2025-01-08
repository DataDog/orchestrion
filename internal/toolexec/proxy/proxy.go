// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package proxy

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type (
	// CommandType represents a Go toolchain command type, such
	// as "compile", "link", etc...
	CommandType int
	// Command represents a Go compilation command
	Command interface {
		// Close invokes all registered OnClose callbacks and releases any resources associated with the
		// command.
		Close(error) error

		// Args are all the command arguments, starting from the Go tool command
		Args() []string
		ReplaceParam(param string, val string) error

		// Type represents the go tool command type (compile, link, asm, etc.)
		Type() CommandType

		// ShowVersion returns true if the command received the `-V=full` argument, signaling it should
		// print its full version information and exit. This feature is used by the go toolchain to
		// create build cache keys, and allows invalidating all build cache when the tooling changes.
		ShowVersion() bool
	}

	// CommandProcessor is a function that takes a command as input and is allowed to modify it or
	// read its data. If it returns an error, the processing chain immediately stops and no further
	// processors will be invoked.
	CommandProcessor[T Command] func(context.Context, T) error

	// command is the default unknown command type
	// Can be used to compose specific Command implementations
	command struct {
		args []string
		// paramPos is the index in args of the *value* provided for the parameter stored in the key
		paramPos map[string]int
		onClose  []func(error) error
	}
)

const (
	CommandTypeOther CommandType = iota
	CommandTypeCompile
	CommandTypeLink
)

// ProcessCommand applies a processor on a command if said command matches
// the input type of said input processor. Nothing happens if the processor does
// not correspond to the provided command type.
func ProcessCommand[T Command](ctx context.Context, cmd Command, p CommandProcessor[T]) error {
	if c, ok := cmd.(T); ok {
		if err := p(ctx, c); err != nil {
			return err
		}
	}
	return nil
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

	return cmd
}

// OnClose registers a callback to be invoked when the command is closed, usually after it has run,
// unless skipping was requested by the integration.
func (cmd *command) OnClose(cb func(error) error) {
	cmd.onClose = append(cmd.onClose, cb)
}

func (cmd *command) Close(err error) error {
	// Run these in reverse order, so it behaves like "defer".
	for idx := len(cmd.onClose) - 1; idx >= 0; idx-- {
		cb := cmd.onClose[idx]
		if err := cb(err); err != nil {
			return err
		}
	}
	return nil
}

// SetFlag replaces the value of the specified flag with the provided one.
// Returns an error if the flag is not present in the current arguments list.
func (cmd *command) SetFlag(flag string, val string) error {
	for arg, idx := range cmd.paramPos {
		if arg == flag || arg == "-"+flag {
			cmd.args[idx+1] = val
			return nil
		}

		f, _, ok := strings.Cut(arg, "=")
		if !ok || (f != flag && f != "-"+flag) {
			continue
		}
		cmd.args[idx] = f + "=" + val
		return nil
	}

	return fmt.Errorf("argument %q not found in %q", flag, cmd.args)
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

func (*command) Type() CommandType {
	return CommandTypeOther
}

func (cmd *command) Args() []string {
	return cmd.args
}

func (*command) ShowVersion() bool {
	return false
}
