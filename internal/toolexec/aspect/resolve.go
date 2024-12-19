// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package aspect

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/DataDog/orchestrion/internal/jobserver/client"
	"github.com/DataDog/orchestrion/internal/jobserver/pkgs"
)

// resolvePackageFiles attempts to retrieve the archive for the designated import path. It attempts
// to locate the archive for `importPath` and its dependencies using `go list`. If that fails, it
// will try to resolve it using `go get`.
func resolvePackageFiles(ctx context.Context, importPath string, workDir string) (map[string]string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	conn, err := client.FromEnvironment(ctx, workDir)
	if err != nil {
		return nil, err
	}

	req := pkgs.NewResolveRequest(cwd, importPath)
	if workDir != "" {
		// Nest the future GOTMPDIR under this $WORK directory, so that builds with `-work` are nested,
		// and the root work tree contains all child work trees involved in resolutions.
		req.TempDir = filepath.Join(workDir, "__tmp__")
	}
	archives, err := client.Request[*pkgs.ResolveRequest, pkgs.ResolveResponse](
		context.Background(),
		conn,
		req,
	)
	if err != nil {
		return nil, err
	}

	// Check for missing archives...
	var found bool
	for ip, arch := range archives {
		if arch == "" {
			return nil, fmt.Errorf("failed to resolve archive for %q", ip)
		}
		if ip == importPath {
			found = true
		}
	}

	if !found {
		return nil, fmt.Errorf("resolution did not include requested package %q", importPath)
	}

	return archives, nil
}
