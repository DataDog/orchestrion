// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package proxy

import (
	"context"
	"errors"
	"path/filepath"
)

// ParseCommand parses the Go tool call and its arguments and returns it as a [Command]. The go tool
// call path should be the first element of args. A nil [Command] may be returned if the command is
// to be ignored (as the result of re-using a previous identical command's side effects).
func ParseCommand(ctx context.Context, importPath string, args []string) (Command, error) {
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
		cmd, err := parseCompileCommand(ctx, importPath, args)
		if cmd == nil || err != nil {
			// There was an error, or we re-used command outputs.
			return nil, err
		}
		return cmd, err
	case CommandTypeLink:
		return parseLinkCommand(ctx, args)
	// We currently don't need to inject other tool calls, so we parse them as generic unsupported commands
	default:
		return &command{args: args}, nil
	}
}

// MustParseCommand calls ParseCommand and exits on error
func MustParseCommand(ctx context.Context, importPath string, args []string) Command {
	cmd, err := ParseCommand(ctx, importPath, args)
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
