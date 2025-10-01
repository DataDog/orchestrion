// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package gomod

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
)

type (
	// File represents some selected entries from the content of a `go.mod` file.
	File struct {
		// Go is the value of the `go` directive.
		Go Version
		// Toolchain is the value of the `toolchain` directive, if present.
		Toolchain Toolchain
		// Require is a list of all `require` directives' contents.
		Require []Require
	}

	// Edit represents an edition that can be made to a `go.mod` file via `go mod edit`.
	Edit interface {
		goModEditFlag() string
	}

	Version   string
	Toolchain string

	// Require represents the target of a `require` directive entry.
	Require struct {
		// Path is the path of the required module.
		Path string
		// Version is the required module's version.
		Version string
	}

	// Replace represents the target of a `replace` directive entry.
	Replace struct {
		// OldPath is the path of the module being replaced.
		OldPath string
		// OldVersion is the version of the module being replaced, if any.
		OldVersion string
		// NewPath is the path of the replacement module.
		NewPath string
		// NewVersion is the version of the replacement module, if any.
		NewVersion string
	}
)

// Parse processes the contents of the designated `go.mod` file using
// `go mod edit -json` and returns the corresponding parsed [goMod].
func Parse(ctx context.Context, modfile string) (File, error) {
	var stdout bytes.Buffer
	if err := Run(ctx, "edit", modfile, &stdout, "-json"); err != nil {
		return File{}, fmt.Errorf("running `go mod edit -json`: %w", err)
	}

	var mod File
	if err := json.NewDecoder(&stdout).Decode(&mod); err != nil {
		return File{}, fmt.Errorf("decoding output of `go mode edit -json`: %w", err)
	}

	return mod, nil
}

// Requires returns true if the `go.mod` file contains a require directive for
// the designated module path.
func (m *File) Requires(path string) (string, bool) {
	for _, r := range m.Require {
		if r.Path == path {
			return r.Version, true
		}
	}
	return "", false
}

// RunGet executes the `go get <modSpecs...>` subcommand with the provided
// module specifications on the designated `go.mod` file.
func RunGet(ctx context.Context, modfile string, modSpecs ...string) error {
	cmd := exec.CommandContext(ctx, "go", "get", "-modfile", modfile)
	cmd.Args = append(cmd.Args, modSpecs...)
	cmd.Env = append(os.Environ(), "GOTOOLCHAIN=local")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// Run executes the `go mod <command> <args...>` subcommand with the
// provided arguments on the designated `go.mod` file, sending standard output
// to the provided writer.
func Run(ctx context.Context, command string, modfile string, stdout io.Writer, args ...string) error {
	cmd := exec.CommandContext(ctx, "go", "mod", command, "-modfile", modfile)
	cmd.Args = append(cmd.Args, args...)
	cmd.Env = append(os.Environ(), "GOTOOLCHAIN=local")
	cmd.Stdin = os.Stdin
	cmd.Stdout = stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// RunEdit makes the specified changes to the `go.mod` file, then runs `go mod tidy` if needed.
// If there is a `vendor` directory, it also runs `go mod vendor` before returning.
func RunEdit(ctx context.Context, modfile string, edits ...Edit) error {
	if len(edits) == 0 {
		// Nothing to do.
		return nil
	}

	editFlags := make([]string, len(edits))
	for i, edit := range edits {
		editFlags[i] = edit.goModEditFlag()
	}
	if err := Run(ctx, "edit", modfile, os.Stdout, editFlags...); err != nil {
		return fmt.Errorf("running `go mod edit %s`: %w", editFlags, err)
	}

	if err := Run(ctx, "tidy", modfile, os.Stdout); err != nil {
		return fmt.Errorf("running `go mod tidy`: %w", err)
	}

	vendorDir := filepath.Join(modfile, "..", "vendor")
	stat, err := os.Stat(vendorDir)
	if errors.Is(err, fs.ErrNotExist) || (err == nil && !stat.IsDir()) {
		//  No `vendor` directory, nothing to do...
		return nil
	}
	if err != nil {
		return fmt.Errorf("checking for vendor directory %q: %w", vendorDir, err)
	}

	if err := Run(ctx, "vendor", modfile, os.Stdout); err != nil {
		return fmt.Errorf("running `go mod vendor`: %w", err)
	}

	return nil
}

func (v Version) goModEditFlag() string {
	return "-go=" + string(v)
}

func (t Toolchain) goModEditFlag() string {
	if t == "" {
		return "-toolchain=none"
	}
	return "-toolchain=" + string(t)
}

func (r Require) goModEditFlag() string {
	return fmt.Sprintf("-require=%s@%s", r.Path, r.Version)
}

func (r Replace) goModEditFlag() string {
	old := r.OldPath
	if r.OldVersion != "" {
		old += "@" + r.OldVersion
	}
	new := r.NewPath
	if r.NewVersion != "" {
		new += "@" + r.NewVersion
	}
	return fmt.Sprintf("-replace=%s=%s", old, new)
}
