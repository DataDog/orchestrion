// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package proxy

import (
	"bytes"
	"errors"
	"fmt"
	"io"
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

		// IsVersion determines whether this is a `-V` invocation of the command, and if so, returns the
		// style string (e.g: "full"). In such cases, the execution flow should be diverted to avoid the
		// majority of processing to instead output the relevant information; as this eventually feeds
		// into the go build cache key.
		IsVersion() (bool, string)

		// Close releases any resources bound to this command.
		Close() error
	}

	// CommandProcessor is a function that takes a command as input
	// and is allowed to modify it or read its data
	CommandProcessor[T Command] func(T) error

	commandFlagSet struct {
		Output  string  `ddflag:"-o"`
		Version *string `ddflag:"-V"`
	}

	CloseCallback func() error

	// command is the default unknown command type
	// Can be used to compose specific Command implementations
	command struct {
		args []string
		// paramPos is the index in args of the *value* provided for the parameter stored in the key
		paramPos map[string]int
		flags    commandFlagSet

		onClose []CloseCallback
	}
)

const (
	CommandTypeOther CommandType = iota
	CommandTypeAsm
	CommandTypeCgo
	CommandTypeCompile
	CommandTypeLink
)

var (
	// ErrSkipCommand is returned by proxy command processors when the processed command should not be
	// run, and instead idempotent success should be claimed.
	ErrSkipCommand = errors.New("skip command")
)

func ProcessAllCommands(cmd Command, p interface {
	ProcessAsm(*AsmCommand) error
	ProcessCgo(*CgoCommand) error
	ProcessCompile(*CompileCommand) error
	ProcessLink(*LinkCommand) error
}) error {
	if err := ProcessCommand(cmd, p.ProcessAsm); err != nil {
		return err
	}
	if err := ProcessCommand(cmd, p.ProcessCgo); err != nil {
		return err
	}
	if err := ProcessCommand(cmd, p.ProcessCompile); err != nil {
		return err
	}
	if err := ProcessCommand(cmd, p.ProcessLink); err != nil {
		return err
	}
	return nil
}

// ProcessCommand applies a processor on a command if said command matches
// the input type of said input processor. Failure to match types is not
// considered to be an error.
func ProcessCommand[T Command](cmd Command, p CommandProcessor[T]) error {
	if c, ok := cmd.(T); ok {
		return p(c)
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

	parseFlags(&cmd.flags, args[1:])

	return cmd
}

func (cmd *command) OnClose(cb CloseCallback) {
	cmd.onClose = append(cmd.onClose, cb)
}

func (cmd *command) Close() error {
	// Traversing in reverse order to preserve sanity...
	for i := len(cmd.onClose) - 1; i >= 0; i-- {
		cb := cmd.onClose[i]
		if err := cb(); err != nil {
			return err
		}
	}
	return nil
}

func (cmd *command) PrependArgs(args ...string) {
	cmd.args = append(
		append(
			append(
				make([]string, 0, len(cmd.args)+len(args)),
				cmd.args[0],
			),
			args...,
		),
		cmd.args[1:]...,
	)
	for pos, v := range cmd.args {
		cmd.paramPos[v] = pos
	}
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

func (cmd *command) IsVersion() (result bool, format string) {
	if result = cmd.flags.Version != nil; result {
		format = *cmd.flags.Version
	}
	return
}

// RunCommand executes the underlying go tool command and forwards the program's standard fluxes
func RunCommand(cmd Command) error {
	args := cmd.Args()
	c := exec.Command(args[0], args[1:]...)

	stderr := bytes.NewBuffer(nil)

	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = io.MultiWriter(stderr, os.Stderr)

	err := c.Run()
	if err, ok := err.(*exec.ExitError); ok && err != nil {
		err.Stderr = stderr.Bytes()
	}
	return err
}

// MustRunCommand is like RunCommand but panics if the command fails to build or run
func MustRunCommand(cmd Command) {
	var exitErr *exec.ExitError
	err := RunCommand(cmd)
	if err == nil {
		return
	}
	if errors.As(err, &exitErr) {
		os.Exit(exitErr.ExitCode())
	}
	panic(err)
}

func (cmd *command) Stage() string {
	// TODO: This may not be correct for some link commands which output in <stage>/exe/<bin>
	return filepath.Base(filepath.Dir(cmd.flags.Output))
}

func (cmd *command) Type() CommandType {
	return CommandTypeOther
}

func (cmd *command) Args() []string {
	return cmd.args
}
