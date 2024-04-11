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

	"github.com/datadog/orchestrion/internal/log"
)

// resolvePackageFiles attempts to retrieve the archive for the designated import path. It attempts
// to locate the archive for `importPath` and its dependencies using `go list` first, which may
// return entries from the GOCACHE. If that fails or a component is stale, `go build` will be
// invoked to create a new archive; and `go list` is then used again to obtain the path of the
// archives.
func resolvePackageFiles(importPath string) (map[string]string, error) {
	// Apply quoting as appropriate to avoid shell interpretation issues...
	toolexec := fmt.Sprintf("%q %q", os.Args[0], os.Args[1])

	attemptedBuild := false // Whether we attempted a build already or not
	for {
		cmd := exec.Command("go", "list", "-toolexec", toolexec, "-json", "-deps", "-export", "--", importPath)
		var stdout bytes.Buffer
		cmd.Stdout = &stdout

		log.Tracef("Attempting to resolve %q using %q\n", importPath, cmd.Args)
		if err := cmd.Run(); err != nil {
			return nil, fmt.Errorf("running %q: %w", cmd.Args, err)
		}

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
		var err error
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

		if err == nil {
			return output, nil
		} else if attemptedBuild {
			return nil, fmt.Errorf("after `go build`: %s", err)
		}

		// Not found -- let's try to `go build` it so it's present and not stale.
		log.Tracef("Attempting to build %q in order to satisfy dependency...\n", importPath)
		if err := exec.Command("go", "build", "-toolexec", toolexec, "--", importPath).Run(); err != nil {
			return nil, fmt.Errorf("building %q: %w", importPath, err)
		}
		attemptedBuild = true
	}
}
