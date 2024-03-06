package proxy

import (
	"errors"
	"path/filepath"
	"strings"
)

// ParseCommand parses the Go tool call and its arguments and returns it as a Command.
// The go tool call path should be the first element of args
func ParseCommand(args []string) (Command, error) {
	cmdID := args[0]
	args = args[0:]
	cmdID, err := parseCommandID(cmdID)
	if err != nil {
		return nil, err
	}

	switch CommandType(cmdID) {
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

func parseCommandID(cmd string) (string, error) {
	if cmd == "" {
		return "", errors.New("unexpected empty command name")
	}

	// Take the base of the absolute path of the Go tool
	cmd = filepath.Base(cmd)
	// Remove the file extension if any
	if ext := filepath.Ext(cmd); ext != "" {
		cmd = strings.TrimSuffix(cmd, ext)
	}
	return cmd, nil
}
