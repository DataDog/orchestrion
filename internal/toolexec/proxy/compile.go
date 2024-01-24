package proxy

import (
	"errors"
	"fmt"
)

type compileFlagSet struct {
	Package   string `ddflag:"-p"`
	ImportCfg string `ddflag:"-importcfg"`
	Output    string `ddflag:"-o"`
	TrimPath  string `ddflag:"-trimpath"`
}

type CompileCommand struct {
	command
	Flags compileFlagSet
}

func (cmd *CompileCommand) Type() CommandType { return CommandTypeCompile }

func (cmd *CompileCommand) Inject(i Injector) {
	i.InjectCompile(cmd)
}

func (f *compileFlagSet) IsValid() bool {
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
	return &cmd, nil
}
