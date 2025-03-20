// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package proxy

import (
	"bytes"
	gocontext "context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/DataDog/orchestrion/internal/files"
	"github.com/DataDog/orchestrion/internal/injector/aspect/context"
	"github.com/DataDog/orchestrion/internal/jobserver/client"
	"github.com/DataDog/orchestrion/internal/jobserver/nbt"
	"github.com/DataDog/orchestrion/internal/toolexec/aspect/linkdeps"
	"github.com/DataDog/orchestrion/internal/toolexec/importcfg"
	"github.com/blakesmith/ar"
	"github.com/rs/zerolog"
)

//go:generate go run github.com/DataDog/orchestrion/internal/toolexec/proxy/generator -command=compile

type compileFlagSet struct {
	Asmhdr      string `ddflag:"-asmhdr"`
	BuildID     string `ddflag:"-buildid"`
	ImportCfg   string `ddflag:"-importcfg"`
	Lang        string `ddflag:"-lang"`
	Output      string `ddflag:"-o"`
	Package     string `ddflag:"-p"`
	ShowVersion bool   `ddflag:"-V"`
}

// CompileCommand represents a go tool `compile` invocation
type CompileCommand struct {
	command
	Files []string
	Flags compileFlagSet
	// WorkDir is the $WORK directory managed by the go toolchain.
	WorkDir string

	// LinkDeps lists all link-time dependencies that must be honored to link a
	// dependent of the built package. If not blank, this is written to disk, then
	// appended to the archive output.
	LinkDeps linkdeps.LinkDeps

	// importPath is the import path of the package being built.
	importPath string
	// finishToken is the token returned by the job server in response to the
	// [nbt.StartRequest] when the operation needs to continue, and that is then
	// forwarded to the [nbt.FinishRequest].
	finishToken string
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

func (cmd *CompileCommand) Close(ctx gocontext.Context, cmdErr error) (err error) {
	defer func() { err = errors.Join(err, cmd.command.Close(ctx, cmdErr)) }()

	if cmdErr == nil {
		// Success so far, we attach link-time dependencies...
		err = cmd.attachLinkDeps(ctx)
	}

	// Notify the job server of the status of the command, and combine with the previous error if any...
	err = errors.Join(err, cmd.notifyJobServer(ctx, errors.Join(cmdErr, err)))

	return err
}

func (cmd *CompileCommand) attachLinkDeps(ctx gocontext.Context) error {
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

	var buf bytes.Buffer
	if err := cmd.LinkDeps.Write(&buf); err != nil {
		return fmt.Errorf("writing "+linkdeps.Filename+": %w", err)
	}

	log := zerolog.Ctx(ctx)
	log.Debug().Str("archive", cmd.Flags.Output).Array(linkdeps.Filename, &cmd.LinkDeps).Msg("Adding " + linkdeps.Filename + " file in archive")

	file, err := os.OpenFile(cmd.Flags.Output, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("opening archive: %w", err)
	}
	defer file.Close()

	wr := ar.NewWriter(file)
	if err := wr.WriteHeader(&ar.Header{Name: linkdeps.Filename, Mode: 0o644, Size: int64(buf.Len())}); err != nil {
		return fmt.Errorf("writing header: %w", err)
	}
	if _, err := wr.Write(buf.Bytes()); err != nil {
		return fmt.Errorf("writing "+linkdeps.Filename+" entry: %w", err)
	}

	return nil
}

func (cmd *CompileCommand) notifyJobServer(ctx gocontext.Context, cmdErr error) error {
	if cmd.finishToken == "" {
		// Nothing to do...
		return nil
	}

	jobs, err := client.FromEnvironment(ctx, cmd.WorkDir)
	if err != nil {
		return err
	}

	var (
		errorMessage *string
		files        map[nbt.Label]string
	)
	if cmdErr != nil {
		msg := cmdErr.Error()
		errorMessage = &msg
	} else {
		files = make(map[nbt.Label]string, 2)
		if filename := cmd.Flags.Output; filename != "" {
			files[nbt.LabelArchive] = filename
		}
		if filename := cmd.Flags.Asmhdr; filename != "" {
			files[nbt.LabelAsmhdr] = filename
		}
	}

	_, err = client.Request(ctx, jobs, nbt.FinishRequest{
		ImportPath:  cmd.importPath,
		FinishToken: cmd.finishToken,
		Files:       files,
		Error:       errorMessage,
	})

	return err
}

// parseCompileCommand parses a [*CompileCommand] from the provided arguments.
// It sends an [nbt.StartRequest] to the job server to determine whether a
// previous execution of the same command has produced re-usable artifacts;
// in which case it copies them into place and returns nil.
func parseCompileCommand(ctx gocontext.Context, importPath string, args []string) (*CompileCommand, error) {
	if len(args) == 0 {
		return nil, errors.New("unexpected number of command arguments")
	}
	cmd := &CompileCommand{command: NewCommand(args), importPath: importPath}
	pos, err := cmd.Flags.parse(args[1:])
	if err != nil {
		return nil, err
	}
	cmd.Files = pos

	if cmd.Flags.ImportCfg == "" {
		return cmd, nil
	}

	// The WorkDir is the parent of the stage directory, which is where the importcfg file is located.
	cmd.WorkDir = filepath.Dir(filepath.Dir(cmd.Flags.ImportCfg))

	jobs, err := client.FromEnvironment(ctx, cmd.WorkDir)
	if err != nil {
		return nil, err
	}

	res, err := client.Request(ctx, jobs, nbt.StartRequest{ImportPath: importPath, BuildID: cmd.Flags.BuildID})
	if err != nil {
		return nil, fmt.Errorf("sending never-build-twice request: %w", err)
	}

	if res.FinishToken != "" {
		cmd.finishToken = res.FinishToken

		imports, err := importcfg.ParseFile(ctx, cmd.Flags.ImportCfg)
		if err != nil {
			return nil, fmt.Errorf("parsing %q: %w", cmd.Flags.ImportCfg, err)
		}

		cmd.LinkDeps, err = linkdeps.FromImportConfig(ctx, &imports)
		if err != nil {
			return nil, fmt.Errorf("reading %s closure from %s: %w", linkdeps.Filename, cmd.Flags.ImportCfg, err)
		}

		return cmd, nil
	}

	if outputFile := cmd.Flags.Output; outputFile != "" {
		filename := res.Files[nbt.LabelArchive]
		if filename == "" {
			return nil, fmt.Errorf("missing %q object in re-usable artifacts", nbt.LabelArchive)
		}
		if err := files.Copy(ctx, filename, outputFile); err != nil {
			return nil, fmt.Errorf("re-using %q object: %w", nbt.LabelArchive, err)
		}
		// We place a "reused" marker next to the output file to identify that it was re-used.
		if err := os.WriteFile(filename+".reused", nil, 0o644); err != nil {
			return nil, fmt.Errorf("creating re-used marker for %s: %w", filename, err)
		}
	}

	if outputFile := cmd.Flags.Asmhdr; outputFile != "" {
		filename := res.Files[nbt.LabelAsmhdr]
		if filename == "" {
			return nil, fmt.Errorf("missing %q object in re-usable artifacts", nbt.LabelAsmhdr)
		}
		if err := files.Copy(ctx, filename, outputFile); err != nil {
			return nil, fmt.Errorf("re-using %q object: %w", nbt.LabelAsmhdr, err)
		}
		// We place a "reused" marker next to the output file to identify that it was re-used.
		if err := os.WriteFile(filename+".reused", nil, 0o644); err != nil {
			return nil, fmt.Errorf("creating re-used marker for %s: %w", filename, err)
		}
	}

	zerolog.Ctx(ctx).Info().Msg("Re-used previously built artifacts from compile command. Returning a nil *CompileCommand.")
	return nil, nil
}

var _ Command = (*CompileCommand)(nil)
