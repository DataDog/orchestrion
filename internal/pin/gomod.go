// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package pin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

type (
	goMod struct {
		Go        goModVersion
		Toolchain goModToolchain
		Require   []goModRequire
		Replace   []goModReplace
	}

	goModVersion   string
	goModToolchain string

	goModRequire struct {
		Path    string
		Version string
	}

	goModReplace struct {
		Old goModModule
		New goModModule
	}

	goModModule struct {
		Path    string
		Version string
	}
)

func parse(modfile string) (goMod, error) {
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

func (m *goMod) requires(path string) bool {
	for _, r := range m.Require {
		if r.Path == path {
			return true
		}
	}
	return false
}

func (m *goMod) replaces(path string) bool {
	for _, r := range m.Replace {
		if r.Old.Path == path {
			return true
		}
	}
	return false
}

func runGoMod(command string, modfile string, stdout io.Writer, args ...string) error {
	cmd := exec.Command("go", "mod", command, "-modfile", modfile)
	cmd.Args = append(cmd.Args, args...)
	cmd.Env = append(os.Environ(), "GOTOOLCHAIN=local")
	cmd.Stdin = os.Stdin
	cmd.Stdout = stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

type goModEdit interface {
	goModEditFlag() string
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
	if os.IsNotExist(err) || (err == nil && !stat.IsDir()) {
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
