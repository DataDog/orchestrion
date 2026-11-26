// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package gomod

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	goversion "go/version"

	"github.com/DataDog/orchestrion/internal/goenv"
)

// AlignWorkGoVersion makes sure the `go` directive of the go.work file enclosing
// the module in moduleDir (if any) is at least as new as the `go` directive of
// the designated go.mod file.
//
// `go mod tidy` raises a module's `go` directive when it acquires a dependency
// requiring a newer language version. If the module is part of a workspace, the
// go.work file must be updated accordingly, or every subsequent `go` command in
// the workspace fails with an error such as:
//
//	go: module . listed in go.work file requires go >= 1.26.0, but go.work lists go 1.26
//
// This notably affects etcd-style monorepos, whose go.work files use the
// two-part form ("1.26"), which does not satisfy a module's three-part
// requirement ("1.26.0").
func AlignWorkGoVersion(ctx context.Context, moduleDir string, modfile string) error {
	workfile, found := findWorkFile(moduleDir)
	if !found {
		// The module is not part of a workspace, nothing to do.
		return nil
	}

	mod, err := Parse(ctx, modfile)
	if err != nil {
		return err
	}

	if workGo, found := readGoDirective(workfile); found && goversion.Compare("go"+workGo, "go"+string(mod.Go)) >= 0 {
		// The go.work file already allows the module's `go` version, nothing to do.
		return nil
	}

	// `go work edit` only reads and rewrites the designated file, so it is safe
	// to use here even while the workspace is in the inconsistent state this
	// call is meant to repair. The explicit file name keeps it independent from
	// any inherited GOWORK value.
	cmd := exec.CommandContext(ctx, "go", "work", "edit", "-go="+string(mod.Go), workfile)
	cmd.Dir = moduleDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("running `go work edit -go=%s %s`: %w", mod.Go, workfile, err)
	}

	return nil
}

// vendorWorkspace runs `go work vendor` if the module of the designated go.mod
// file belongs to a workspace that was vendored with `go work vendor`, and
// reports whether it did so.
//
// Such a workspace must be re-vendored as a whole: `go mod vendor` (which runs
// with GOWORK=off, see [commandEnv]) would replace the workspace's vendor
// directory with one for this module alone when go.work is next to go.mod, and
// would not update it at all when go.work is in a parent directory.
func vendorWorkspace(ctx context.Context, modfile string) (bool, error) {
	moduleDir := filepath.Dir(modfile)
	workfile, err := goenv.GOWORK(moduleDir)
	if err != nil {
		return false, err
	}
	if workfile == "" || workfile == "off" {
		// Not in workspace mode (`go env GOWORK` prints "off" for GOWORK=off), nothing to do.
		return false, nil
	}

	workDir := filepath.Dir(workfile)
	// `go work vendor` starts vendor/modules.txt with this line; `go mod vendor` does not.
	if content, _ := os.ReadFile(filepath.Join(workDir, "vendor", "modules.txt")); !strings.HasPrefix(string(content), "## workspace") {
		return false, nil
	}

	// `go work vendor` fails if `go mod tidy` raised the module's `go` directive
	// above the one in go.work.
	if err := AlignWorkGoVersion(ctx, moduleDir, modfile); err != nil {
		return false, err
	}

	cmd := exec.CommandContext(ctx, "go", "work", "vendor")
	cmd.Dir = workDir
	cmd.Env = append(os.Environ(), "GOTOOLCHAIN=local", "GOWORK="+workfile)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return false, fmt.Errorf("running `go work vendor`: %w", err)
	}

	return true, nil
}

// findWorkFile looks for a go.work file in moduleDir or any of its parent
// directories, mirroring the go command's workspace discovery.
func findWorkFile(moduleDir string) (string, bool) {
	for dir := moduleDir; ; dir = filepath.Dir(dir) {
		workfile := filepath.Join(dir, "go.work")
		if _, err := os.Stat(workfile); err == nil {
			return workfile, true
		}
		if parent := filepath.Dir(dir); parent == dir {
			// Reached the root of the file system without finding a go.work file.
			return "", false
		}
	}
}

// readGoDirective returns the value of the `go` directive in the designated
// go.work file. The go.work format is a simple modfile, so the directive is
// read directly rather than consulting the go command, which would have to
// load the whole workspace (and could not do so while it is inconsistent).
func readGoDirective(workfile string) (string, bool) {
	file, err := os.Open(workfile)
	if err != nil {
		// The file exists but cannot be read; let `go work edit` surface the
		// underlying error instead.
		return "", false
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		// The go.work syntax allows a trailing `//` comment on any line.
		line, _, _ := strings.Cut(scanner.Text(), "//")
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[0] == "go" {
			return fields[1], true
		}
	}

	return "", false
}
