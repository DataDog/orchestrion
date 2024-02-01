package proxy

import (
	"errors"
	"path/filepath"
)

type linkFlagSet struct {
	BuildMode string `ddflag:"-buildmode"`
	ImportCfg string `ddflag:"-importcfg"`
	Output    string `ddflag:"-o"`
}

type LinkCommand struct {
	command
	Flags linkFlagSet
}

func (cmd *LinkCommand) Type() CommandType {
	return CommandTypeLink
}

func (cmd *LinkCommand) Stage() string {
	return filepath.Base(filepath.Dir(filepath.Dir(cmd.Flags.Output)))
}

func parseLinkCommand(args []string) (Command, error) {
	if len(args) == 0 {
		return nil, errors.New("unexpected number of command arguments")
	}
	flags := &linkFlagSet{}
	parseFlags(flags, args[1:])
	return &LinkCommand{command: NewCommand(args), Flags: *flags}, nil
}
