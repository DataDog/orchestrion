// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package pin

import (
	"bytes"
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
	// goMod represents some selected entries from the content of a `go.mod` file.
	goMod struct {
		// Go is the value of the `go` directive.
		Go goModVersion
		// Toolchain is the value of the `toolchain` directive, if present.
		Toolchain goModToolchain
		// Require is a list of all `require` directives' contents.
		Require []goModRequire
	}

	// goModEdit represents an edition that can be made to a `go.mod` file via `go mod edit`.
	goModEdit interface {
		goModEditFlag() string
	}

	goModVersion   string
	goModToolchain string

	// goModRequire represents the target of a `require` directive entry.
	goModRequire struct {
		// Path is the path of the required module.
		Path string
		// Version is the required module's version.
		Version string
	}
)

// parseGoMod processes the contents of the designated `go.mod` file using
// `go mod edit -json` and returns the corresponding parsed [goMod].
func parseGoMod(modfile string) (goMod, error) {
	var stdout bytes.Buffer
	if err := runGoMod("edit", modfile, &stdout, "-json"); err != nil {
		return goMod{}, fmt.Errorf("running `go mod edit -json`: %w", err)
	}

	var mod goMod
	if err := json.NewDecoder(&stdout).Decode(&mod); err != nil {
		return goMod{}, fmt.Errorf("decoding output of `go mode edit -json`: %w", err)
	}

	return mod, nil
}

// requires returns true if the `go.mod` file contains a require directive for
// the designated module path.
func (m *goMod) requires(path string) bool {
	for _, r := range m.Require {
		if r.Path == path {
			return true
		}
	}
	return false
}

// runGoMod executes the `go mod <command> <args...>` subcommand with the
// provided arguments on the designated `go.mod` file, sending standard output
// to the provided writer.
func runGoMod(command string, modfile string, stdout io.Writer, args ...string) error {
	cmd := exec.Command("go", "mod", command, "-modfile", modfile)
	cmd.Args = append(cmd.Args, args...)
	cmd.Env = append(os.Environ(), "GOTOOLCHAIN=local")
	cmd.Stdin = os.Stdin
	cmd.Stdout = stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// runGoModEdit makes the specified changes to the `go.mod` file, then runs `go mod tidy` if needed.
// If there is a `vendor` directory, it also runs `go mod vendor` before returning.
func runGoModEdit(modfile string, edits ...goModEdit) error {
	if len(edits) == 0 {
		// Nothing to do.
		return nil
	}

	editFlags := make([]string, len(edits))
	for i, edit := range edits {
		editFlags[i] = edit.goModEditFlag()
	}
	if err := runGoMod("edit", modfile, os.Stdout, editFlags...); err != nil {
		return fmt.Errorf("running `go mod edit %s`: %w", editFlags, err)
	}

	if err := runGoMod("tidy", modfile, os.Stdout); err != nil {
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

	if err := runGoMod("vendor", modfile, os.Stdout); err != nil {
		return fmt.Errorf("running `go mod vendor`: %w", err)
	}

	return nil
}

func (v goModVersion) goModEditFlag() string {
	return fmt.Sprintf("-go=%s", string(v))
}

func (t goModToolchain) goModEditFlag() string {
	if t == "" {
		return "-toolchain=none"
	}
	return fmt.Sprintf("-toolchain=%s", string(t))
}

func (r goModRequire) goModEditFlag() string {
	return fmt.Sprintf("-require=%s@%s", r.Path, r.Version)
}
