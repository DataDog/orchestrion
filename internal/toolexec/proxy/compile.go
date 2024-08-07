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
	Flags compileFlagSet
	// SourceDir is the directory containing source files that are in the command's inputs.
	SourceDir string
	// WorkDir is the $WORK directory managed by the go toolchain.
	WorkDir string
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
	paramIdx := len(cmd.args)
	cmd.args = append(cmd.args, files...)
	for i, f := range files {
		cmd.paramPos[f] = paramIdx + i
	}
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

	// The ImportCfg file is rooted in the stage directory
	stageDir := filepath.Dir(cmd.Flags.ImportCfg)
	// The WorkDir is the parent of the stage directory
	cmd.WorkDir = filepath.Dir(stageDir)
	// NOTE: Some commands just print the tool version, in which case no go file will be provided as arg
	for _, f := range files {
		if dir := filepath.Dir(f); dir != stageDir {
			cmd.SourceDir = dir
			break
		}
	}
	return &cmd, nil
}
