// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package proxy

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

type compileFlagSet struct {
	Package   string `ddflag:"-p"`
	ImportCfg string `ddflag:"-importcfg"`
	Output    string `ddflag:"-o"`
	TrimPath  string `ddflag:"-trimpath"`
}

// CompileCommand represents a go tool `compile` invocation
type CompileCommand struct {
	command
	Flags    compileFlagSet
	BuildDir string
}

func (cmd *CompileCommand) Type() CommandType { return CommandTypeCompile }

// GoFiles returns the list of Go files passed as arguments to cmd
func (cmd *CompileCommand) GoFiles() []string {
	files := make([]string, 0, len(cmd.args))
	for _, path := range cmd.args {
		if !strings.HasSuffix(path, ".go") {
			continue
		}
		files = append(files, path)
	}

	return files
}

// AddFiles adds the provided go files paths to the list of Go files passed
// as arguments to cmd
func (cmd *CompileCommand) AddFiles(files []string) {
	paramIdx := len(cmd.paramPos)

	for i, f := range files {
		cmd.paramPos[f] = paramIdx + i
	}
}

// ReplaceGoFile is a convenience wrapper around Command.ReplaceParam to
// replace a Go file by another in the cmd arguments
func (cmd *CompileCommand) ReplaceGoFile(old, new string) error {
	if !strings.HasSuffix(old, ".go") {
		return fmt.Errorf("%s is not a Go file", old)
	}
	if !strings.HasSuffix(new, ".go") {
		return fmt.Errorf("%s is not a Go file", new)
	}

	return cmd.ReplaceParam(old, new)
}

func (f *compileFlagSet) Valid() bool {
	return f.Package != "" && f.Output != "" && f.ImportCfg != "" && f.TrimPath != ""
}

func (f *compileFlagSet) String() string {
	return fmt.Sprintf("-p=%q -o=%q -importcfg=%q", f.Package, f.Output, f.ImportCfg)
}

func parseCompileCommand(args []string) (Command, error) {
	if len(args) == 0 {
		return nil, errors.New("unexpected number of command arguments")
	}
	cmd := CompileCommand{command: NewCommand(args)}
	parseFlags(&cmd.Flags, args[1:])
	files := cmd.GoFiles()
	// Some commands just print the tool version, in which case no go file will be provided as arg
	if len(files) > 0 {
		cmd.BuildDir = filepath.Dir(files[0])
	}
	return &cmd, nil
}
