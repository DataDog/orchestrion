// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package proxy

import (
	"errors"
	"path"
)

type asmFlagSet struct {
	Package string `ddflag:"-p"`
	Output  string `ddflag:"-o"`
}

type AsmCommand struct {
	command
	Flags    asmFlagSet
	StageDir string
}

func (cmd *AsmCommand) Type() CommandType { return CommandTypeAsm }

func (cmd *AsmCommand) SourceFiles() []string {
	result := make([]string, 0, len(cmd.Args()))

	for _, arg := range cmd.Args() {
		if path.Ext(arg) == ".s" {
			result = append(result, arg)
		}
	}

	return result
}

func parseAsmCommand(args []string) (Command, error) {
	if len(args) == 0 {
		return nil, errors.New("unexpected number of command arguments")
	}

	cmd := AsmCommand{command: NewCommand(args)}
	parseFlags(&cmd.Flags, args[1:])

	if cmd.Flags.Output != "" {
		cmd.StageDir = path.Dir(cmd.Flags.Output)
	}

	return &cmd, nil
}
