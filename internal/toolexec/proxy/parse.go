// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package proxy

import (
	"errors"
	"path/filepath"
)

// ParseCommand parses the Go tool call and its arguments and returns it as a Command.
// The go tool call path should be the first element of args
func ParseCommand(args []string) (Command, error) {
	if len(args) == 0 {
		return nil, errors.New("unexpected empty command arguments")
	}

	cmdID := args[0]
	args = args[0:]
	cmdType, err := parseCommandID(cmdID)
	if err != nil {
		return nil, err
	}

	switch cmdType {
	case CommandTypeCompile:
		return parseCompileCommand(args)
	case CommandTypeLink:
		return parseLinkCommand(args)
	// We currently don't need to inject other tool calls, so we parse them as generic unsupported commands
	default:
		return &command{args: args}, nil
	}
}

// MustParseCommand calls ParseCommand and exits on error
func MustParseCommand(args []string) Command {
	cmd, err := ParseCommand(args)
	if err != nil {
		panic(err)
	}

	return cmd
}

func parseCommandID(cmd string) (CommandType, error) {
	if cmd == "" {
		return CommandTypeOther, errors.New("unexpected empty command name")
	}

	// Take the base of the absolute path of the Go tool
	cmd = filepath.Base(cmd)
	// Depending on the architecture/environment, go tools may have extensions. Remove the extension - if any
	if ext := filepath.Ext(cmd); ext != "" {
		cmd = cmd[:len(cmd)-len(ext)]
	}

	var cmdType CommandType
	switch cmd {
	case "compile":
		cmdType = CommandTypeCompile
	case "link":
		cmdType = CommandTypeLink
	default:
		cmdType = CommandTypeOther
	}
	return cmdType, nil
}
