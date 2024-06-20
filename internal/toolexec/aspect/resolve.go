// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package aspect

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/datadog/orchestrion/internal/goflags"
	"github.com/datadog/orchestrion/internal/log"
)

// resolvePackageFiles attempts to retrieve the archive for the designated import path. It attempts
// to locate the archive for `importPath` and its dependencies using `go list`. If that fails, it
// will try to resolve it using `go get`.
func resolvePackageFiles(importPath string) (map[string]string, error) {
	// Apply quoting as appropriate to avoid shell interpretation issues...
	toolexec := fmt.Sprintf("%q %q", os.Args[0], os.Args[1])

	// Retrieve parent Go command flags
	args, err := prepareGoCommandArgs("list", "-toolexec", toolexec, "-json", "-deps", "-export", "--", importPath)
	if err != nil {
		return nil, fmt.Errorf("preparing go command %v: %w", args, err)
	}

	cmd := exec.Command("go", args...)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	log.Tracef("Attempting to resolve %q using %q\n", importPath, cmd.Args)
	if err = cmd.Run(); err != nil {
		return nil, fmt.Errorf("running %q: %w", cmd.Args, err)
	}
	log.Tracef("Command successful, parsing output...\n")

	type listItem struct {
		ImportPath string // The import path of the package
		Export     string // The path to its archive, if any
		BuildID    string // The build ID for the package
		Standard   bool   // Whether this is from the standard library
	}
	var items []listItem
	dec := json.NewDecoder(&stdout)
	for {
		var item listItem
		if err := dec.Decode(&item); err == io.EOF {
			break
		} else if err != nil {
			return nil, fmt.Errorf("parsing `go list` output: %w", err)
		}
		items = append(items, item)
	}

	output := make(map[string]string, len(items))
	for _, item := range items {
		if item.Standard && item.ImportPath == "unsafe" && item.Export == "" {
			// Special-casing "unsafe", because it's not provided like other modules
			continue
		}
		if item.Export == "" {
			err = errors.Join(err, fmt.Errorf("%s (%s) has no export file", item.ImportPath, item.BuildID))
			continue
		}
		output[item.ImportPath] = item.Export
	}

	if err != nil {
		return nil, err
	}
	return output, nil
}

// prepareGoCommandArgs injects the parent Go command's flags into the provided arguments
// The result can be passed as args to a Go invocation through exec.Command()
func prepareGoCommandArgs(cmd string, args ...string) ([]string, error) {
	flags, err := goflags.Flags()
	if err != nil {
		return nil, fmt.Errorf("retrieving go command flags: %w", err)
	}
	slice := flags.Slice()

	compound := make([]string, 1, 1+len(slice)+len(args))
	compound[0] = cmd
	compound = append(compound, slice...)
	compound = append(compound, args...)
	return compound, nil
}
