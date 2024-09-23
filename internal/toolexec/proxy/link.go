// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package proxy

import (
	"errors"
	"path/filepath"
)

//go:generate go run github.com/DataDog/orchestrion/internal/toolexec/proxy/generator -command=link

type linkFlagSet struct {
	BuildMode   string `ddflag:"-buildmode"`
	ImportCfg   string `ddflag:"-importcfg"`
	Output      string `ddflag:"-o"`
	ShowVersion bool   `ddflag:"-V"`
}

// LinkCommand represents a go tool `link` invocation
type LinkCommand struct {
	command
	Flags linkFlagSet
	// WorkDir is the $WORK directory managed by the go toolchain.
	WorkDir string
}

func (*LinkCommand) Type() CommandType {
	return CommandTypeLink
}

func (cmd *LinkCommand) ShowVersion() bool {
	return cmd.Flags.ShowVersion
}

func (cmd *LinkCommand) Stage() string {
	return filepath.Base(filepath.Dir(filepath.Dir(cmd.Flags.Output)))
}

func parseLinkCommand(args []string) (Command, error) {
	if len(args) == 0 {
		return nil, errors.New("unexpected number of command arguments")
	}
	flags := &linkFlagSet{}
	if _, err := flags.parse(args[1:]); err != nil {
		return nil, err
	}

	// The WorkDir is the parent of the stage dir, and the ImportCfg file is directly in the stage dir.
	workDir := filepath.Dir(filepath.Dir(flags.ImportCfg))

	return &LinkCommand{command: NewCommand(args), Flags: *flags, WorkDir: workDir}, nil
}
