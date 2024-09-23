// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package aspect

import (
	"context"
	"fmt"
	"os"

	"github.com/DataDog/orchestrion/internal/goflags"
	"github.com/DataDog/orchestrion/internal/jobserver/client"
	"github.com/DataDog/orchestrion/internal/jobserver/pkgs"
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

	archives, err := client.Request[*pkgs.ResolveRequest, pkgs.ResolveResponse](
		context.Background(),
		conn,
		pkgs.NewResolveRequest(cwd, flags.Slice(), importPath),
	)
	if err != nil {
		return nil, err
	}

	// Check for missing archives...
	for ip, arch := range archives {
		if arch == "" {
			return nil, fmt.Errorf("failed to resolve archive for %q", ip)
		}
	}

	return archives, nil
}
