// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package proxy

import (
	"errors"
	"path/filepath"
	"strings"
)

//go:generate go run github.com/datadog/orchestrion/internal/toolexec/proxy/generator -command=compile

type compileFlagSet struct {
	Package     string `ddflag:"-p"`
	ImportCfg   string `ddflag:"-importcfg"`
	Output      string `ddflag:"-o"`
	Lang        string `ddflag:"-lang"`
	ShowVersion bool   `ddflag:"-V"`
}

// CompileCommand represents a go tool `compile` invocation
type CompileCommand struct {
	command
	Flags compileFlagSet
	Files []string
	// WorkDir is the $WORK directory managed by the go toolchain.
	WorkDir string
}

func (*CompileCommand) Type() CommandType { return CommandTypeCompile }

func (c *CompileCommand) ShowVersion() bool {
	return c.Flags.ShowVersion
}

// GoFiles returns the list of Go files passed as arguments to cmd
func (cmd *CompileCommand) GoFiles() []string {
	files := make([]string, 0, len(cmd.Files))
	for _, path := range cmd.Files {
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
	cmd.Files = append(cmd.Files, files...)
	for i, f := range files {
		cmd.paramPos[f] = paramIdx + i
	}
}

func parseCompileCommand(args []string) (Command, error) {
	if len(args) == 0 {
		return nil, errors.New("unexpected number of command arguments")
	}
	cmd := CompileCommand{command: NewCommand(args)}
	pos, err := cmd.Flags.parse(args[1:])
	if err != nil {
		return nil, err
	}
	cmd.Files = pos

	if cmd.Flags.ImportCfg != "" {
		// The WorkDir is the parent of the stage directory, which is where the importcfg file is located.
		cmd.WorkDir = filepath.Dir(filepath.Dir(cmd.Flags.ImportCfg))
	}

	return &cmd, nil
}
