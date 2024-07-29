// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package aspect

import (
	"context"
	"fmt"
	"os"

	"github.com/datadog/orchestrion/internal/goflags"
	"github.com/datadog/orchestrion/internal/jobserver/client"
	"github.com/datadog/orchestrion/internal/jobserver/pkgs"
)

// resolvePackageFiles attempts to retrieve the archive for the designated import path. It attempts
// to locate the archive for `importPath` and its dependencies using `go list`. If that fails, it
// will try to resolve it using `go get`.
func resolvePackageFiles(importPath string, workDir string) (map[string]string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	conn, err := client.FromEnvironment(workDir)
	if err != nil {
		return nil, err
	}

	flags, err := goflags.Flags()
	if err != nil {
		return nil, fmt.Errorf("retrieving go command flags: %w", err)
	}
	slice := flags.Slice()

	// Apply quoting as appropriate to avoid shell interpretation issues...
	toolexec := fmt.Sprintf("%q %q", os.Args[0], os.Args[1])

	return client.Request[*pkgs.ResolveRequest, pkgs.ResolveResponse](
		context.Background(),
		conn,
		&pkgs.ResolveRequest{
			Dir: cwd,
			Env: os.Environ(),
			BuildFlags: append(
				append(
					make([]string, 0, len(slice)+1),
					slice...,
				),
				fmt.Sprintf("-toolexec=%s", toolexec),
			),
			Patterns: []string{importPath},
		},
	)
}
