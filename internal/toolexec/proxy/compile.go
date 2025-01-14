// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package proxy

import (
	gocontext "context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/DataDog/orchestrion/internal/injector/aspect/context"
	"github.com/DataDog/orchestrion/internal/toolexec/aspect/linkdeps"
	"github.com/DataDog/orchestrion/internal/toolexec/importcfg"
	"github.com/rs/zerolog"
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

	// Link-time dependencies that must be honored to link a dependent of the
	// built package. If not blank, this is written to disk, then appended to the
	// archive output.
	LinkDeps linkdeps.LinkDeps
}

func (*CompileCommand) Type() CommandType { return CommandTypeCompile }

func (c *CompileCommand) ShowVersion() bool {
	return c.Flags.ShowVersion
}

// TestMain returns true if the compiled package name is "main" and all source
// Go files are rooted in the same directory as the importcfg file. This
// indicates the package being compiled is a synthetic "main" package generated
// by `go test`. For more accurate readings, users should also validate the
// declared package import path ends in `.test`.
func (c *CompileCommand) TestMain() bool {
	if c.Flags.Package != "main" {
		return false
	}

	stageDir := filepath.Dir(c.Flags.ImportCfg)
	for _, f := range c.GoFiles() {
		if filepath.Dir(f) != stageDir {
			return false
		}
	}

	return true
}

func (cmd *CompileCommand) SetLang(to context.GoLangVersion) error {
	if to.IsAny() {
		// No minimal language requirement change, nothing to do...
		return nil
	}

	if cmd.Flags.Lang == "" {
		// No language level was specified, so anything the compiler can do is possible...
		return nil
	}

	if curr, _ := context.ParseGoLangVersion(cmd.Flags.Lang); context.Compare(curr, to) >= 0 {
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

func (cmd *CompileCommand) Close(ctx gocontext.Context) (err error) {
	defer func() { err = errors.Join(err, cmd.command.Close(ctx)) }()

	if cmd.LinkDeps.Empty() {
		return nil
	}

	if _, err := os.Stat(cmd.Flags.Output); errors.Is(err, os.ErrNotExist) {
		// Already failing, not doing anything...
		return nil
	} else if err != nil {
		return err
	}

	orchestrionDir := filepath.Join(cmd.Flags.Output, "..", "orchestrion")
	if err := os.MkdirAll(orchestrionDir, 0o755); err != nil {
		return fmt.Errorf("mkdir %q: %w", orchestrionDir, err)
	}

	linkDepsFile := filepath.Join(orchestrionDir, linkdeps.Filename)
	if err := cmd.LinkDeps.WriteFile(linkDepsFile); err != nil {
		return fmt.Errorf("writing %s file: %w", linkdeps.Filename, err)
	}

	log := zerolog.Ctx(ctx)
	log.Debug().Str("archive", cmd.Flags.Output).Array(linkdeps.Filename, &cmd.LinkDeps).Msg("Adding " + linkdeps.Filename + " file in archive")

	child := exec.Command("go", "tool", "pack", "r", cmd.Flags.Output, linkDepsFile)
	if err := child.Run(); err != nil {
		return fmt.Errorf("running %q: %w", child.Args, err)
	}
	return nil
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

		imports, err := importcfg.ParseFile(cmd.Flags.ImportCfg)
		if err != nil {
			return nil, fmt.Errorf("parsing %q: %w", cmd.Flags.ImportCfg, err)
		}

		cmd.LinkDeps, err = linkdeps.FromImportConfig(&imports)
		if err != nil {
			return nil, fmt.Errorf("reading %s closure from %s: %w", linkdeps.Filename, cmd.Flags.ImportCfg, err)
		}
	}

	return &cmd, nil
}

var _ Command = (*CompileCommand)(nil)
