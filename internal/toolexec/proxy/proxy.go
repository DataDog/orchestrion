package proxy

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
)

type (
	// CommandType represents a Go compilation command type
	// Currently support types are:
	// - compile
	// - link
	// - unknown
	CommandType string
	// Command represents a Go compilation command
	Command interface {
		Inject(injector Injector)
		MustRun()
		ReplaceParam(param string, val string) error
		Stage() string
		Type() CommandType
	}

	// Injector visits Command types
	Injector interface {
		InjectCompile(*CompileCommand)
		InjectLink(*LinkCommand)
	}

	commandFlagSet struct {
		Output string `ddflag:"-o"`
	}

	// command is the default unknown command type
	// Can be used to compose specific Command implementations
	command struct {
		args []string
		// paramPos is the index in args of the *value* provided for the parameter stored in the key
		paramPos map[string]int
		flags    commandFlagSet
	}
)

const (
	CommandTypeUnknown CommandType = "unknown"
	CommandTypeCompile             = "compile"
	CommandTypeLink                = "link"
)

func NewCommand(args []string) command {
	cmd := command{
		args:     args,
		paramPos: make(map[string]int),
	}
	for pos, v := range args[1:] {
		cmd.paramPos[v] = pos + 1
	}

	parseFlags(&cmd.flags, args)

	return cmd
}

func (cmd *command) ReplaceParam(param string, val string) error {
	i, ok := cmd.paramPos[param]
	if !ok {
		return fmt.Errorf("%s not found", param)
	}
	cmd.args[i] = val
	delete(cmd.paramPos, param)
	cmd.paramPos[val] = i
	return nil
}

func (cmd *command) MustRun() {
	c := exec.Command(cmd.args[0], cmd.args[1:]...)
	if c == nil {
		log.Printf("%v", fmt.Errorf("command couldn't build"))
		os.Exit(1)
	}

	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	var exitErr *exec.ExitError
	if err := c.Run(); err != nil {
		if errors.As(err, &exitErr) {
			os.Exit(exitErr.ExitCode())
		} else {
			log.Fatalln(err)
		}
	}
}

func (cmd *command) Stage() string {
	return filepath.Base(filepath.Dir(cmd.flags.Output))
}

func (cmd *command) Type() CommandType {
	return CommandTypeUnknown
}

func (cmd *command) Inject(Injector) {}
