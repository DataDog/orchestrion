// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package proxy

import (
	"errors"
	"path"
)

type cgoFlagSet struct {
	ObjDir     string `ddflag:"-objdir"`
	ImportPath string `ddflag:"-importpath"`

	// - OR -

	DynPackage string `ddflag:"-dynpackage"`
	DynImport  string `ddflag:"-dynimport"`
	DynOut     string `ddflag:"-dynout"`
}

type CgoCommand struct {
	command
	Flags    cgoFlagSet
	StageDir string
}

func (cmd *CgoCommand) Type() CommandType { return CommandTypeCgo }

func (cmd *CgoCommand) Stage() string {
	return path.Base(cmd.StageDir)
}

func (cmd *CgoCommand) GoFiles() []string {
	result := make([]string, 0, len(cmd.Args()))
	for _, arg := range cmd.Args() {
		if path.Ext(arg) == ".go" {
			result = append(result, arg)
		}
	}
	return result
}

func parseCgoCommand(args []string) (Command, error) {
	if len(args) == 0 {
		return nil, errors.New("unexpected number of command arguments")
	}

	cmd := CgoCommand{command: NewCommand(args)}
	parseFlags(&cmd.Flags, args[1:])

	if cmd.Flags.ObjDir != "" {
		cmd.StageDir = cmd.Flags.ObjDir
	} else if cmd.Flags.DynOut != "" {
		cmd.StageDir = path.Dir(cmd.Flags.DynOut)
	}

	return &cmd, nil
}
