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
		Args() []string
		ReplaceParam(param string, val string) error
		Stage() string
		Type() CommandType
	}

	// CommandProcessor is a function that takes a command as input
	// and is allowed to modify it or read its data
	CommandProcessor[T Command] func(T)

	commandFlagSet struct {
		Output string `ddflag:"-o"`
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

func RunCommand(cmd Command) error {
	args := cmd.Args()
	c := exec.Command(args[0], args[1:]...)
	if c == nil {
		return fmt.Errorf("command couldn't build")
	}

	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr

	return c.Run()
}

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
	return filepath.Base(filepath.Dir(cmd.flags.Output))
}

func (cmd *command) Type() CommandType {
	return CommandTypeOther
}

func (cmd *command) Args() []string {
	return cmd.args
}
