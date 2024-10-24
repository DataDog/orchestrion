// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package proxy

import (
	"errors"
	"path/filepath"
	"strings"

	"github.com/DataDog/orchestrion/internal/injector/aspect/context"
)

//go:generate go run github.com/DataDog/orchestrion/internal/toolexec/proxy/generator -command=compile

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

func (cmd *CompileCommand) SetLang(to context.GoLang) error {
	if to.IsAny() {
		// No minimal language requirement change, nothing to do...
		return nil
	}

	if cmd.Flags.Lang == "" {
		// No language level was specified, so anything the compiler can do is possible...
		return nil
	}

	if curr, _ := context.ParseGoLang(cmd.Flags.Lang); context.Compare(curr, to) >= 0 {
		// Minimum language requirement from injected code is already met, nothing to do...
		return nil
	}

	if err := cmd.SetFlag("-lang", to.String()); err != nil {
		return err
	}
	cmd.Flags.Lang = to.String()
	return nil
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

func parseCompileCommand(args []string) (*CompileCommand, error) {
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

var _ Command = (*CompileCommand)(nil)
