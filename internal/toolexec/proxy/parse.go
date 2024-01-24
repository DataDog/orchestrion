package proxy

import (
	"errors"
	"path/filepath"
	"strings"
)

type (
	FlagSet interface {
		String()
	}

	parseCommandFunc func([]string) (Command, error)
)

var commandParserMap = map[string]parseCommandFunc{
	"compile": parseCompileCommand,
	"link":    parseLinkCommand,
}

// Walk the list of arguments and add the go source files and the arg slice
// index to returned map.
func goFilesFromArgs(args []string) map[string]int {
	goFiles := make(map[string]int)
	for i, src := range args {
		// Only consider args ending with the Go file extension and assume they
		// are Go files.
		if !strings.HasSuffix(src, ".go") {
			continue
		}
		// Save the position of the source file in the argument list
		// to later change it if it gets instrumented.
		goFiles[src] = i
	}
	return goFiles
}

// Update the argument list by replacing source files that were instrumented.
func updateArgs(args []string, argIndices map[string]int, written map[string]string) {
	for src, dest := range written {
		argIndex := argIndices[src]
		args[argIndex] = dest
	}
}

// ParseCommand returns the command and arguments. The command should be the first argument (not the current program
// invocation)
func ParseCommand(args []string) (Command, error) {
	cmdId := args[0]
	args = args[0:]
	cmdId, err := parseCommandID(cmdId)
	if err != nil {
		return nil, err
	}

	if commandParser, exists := commandParserMap[cmdId]; exists {
		cmd, err := commandParser(args)
		return cmd, err
	} else {
		return &command{args: args}, nil
	}
}

func parseCommandID(cmd string) (string, error) {
	// It mustn't be empty
	if cmd == "" {
		return "", errors.New("unexpected empty command name")
	}

	// Take the base of the absolute path of the go tool
	cmd = filepath.Base(cmd)
	// Remove the file extension if any
	if ext := filepath.Ext(cmd); ext != "" {
		cmd = strings.TrimSuffix(cmd, ext)
	}
	return cmd, nil
}
