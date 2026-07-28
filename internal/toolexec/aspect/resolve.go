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

	"github.com/DataDog/dd-trace-go/v2/ddtrace/tracer"
	"github.com/DataDog/orchestrion/internal/jobserver/client"
	"github.com/DataDog/orchestrion/internal/jobserver/pkgs"
	"github.com/DataDog/orchestrion/internal/toolexec/importcfg"
)

// resolvePackageFiles attempts to retrieve the archive for the designated import path. It attempts
// to locate the archive for `importPath` and its dependencies using `go list`. If that fails, it
// will try to resolve it using `go get`.
func resolvePackageFiles(ctx context.Context, importPath string, workDir string) (_ pkgs.ResolveResponse, err error) {
	return resolvePackageFilesForTest(ctx, importPath, "", workDir)
}

func resolvePackageFilesForTest(ctx context.Context, importPath string, testVariantFor string, workDir string) (_ pkgs.ResolveResponse, err error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "aspect.resolvePackageFiles",
		tracer.ResourceName(importPath),
	)
	defer func() { span.Finish(tracer.WithError(err)) }()

	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	conn, err := client.FromEnvironment(ctx, workDir)
	if err != nil {
		return nil, err
	}

	req := pkgs.NewResolveRequest(cwd, importPath)
	req.TestVariantFor = testVariantFor
	if workDir != "" {
		// Nest the future GOTMPDIR under this $WORK directory, so that builds with `-work` are nested,
		// and the root work tree contains all child work trees involved in resolutions.
		req.TempDir = filepath.Join(workDir, "__tmp__")
	}
	archives, err := client.Request(
		ctx,
		conn,
		req,
	)
	if err != nil {
		return nil, err
	}

	// Check for missing archives...
	var found bool
	for ip, arch := range archives {
		if arch.ExportFile == "" {
			return nil, fmt.Errorf("no archive found for %q", ip)
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

func rejectSyntheticVariantDependency(parent string, dependency string, testVariantFor string, requiresRebuild bool, archives pkgs.ResolveResponse) error {
	if parent == "" || !requiresRebuild || !responseHasTestVariants(archives, testVariantFor) {
		return nil
	}
	return fmt.Errorf("synthetic dependency %q from archive %q requires a test variant for %q; the parent archive was compiled without this edge in Go's package graph and cannot safely use the variant", dependency, parent, testVariantFor)
}

func responseHasTestVariants(archives pkgs.ResolveResponse, testVariantFor string) bool {
	for _, archive := range archives {
		if archive.ForTest == testVariantFor && testVariantFor != "" {
			return true
		}
	}
	return false
}

func mergeResolvedArchives(reg *importcfg.ImportConfig, archives pkgs.ResolveResponse, testVariantFor string) (map[string]string, error) {
	changed := make(map[string]string)
	for importPath, archive := range archives {
		if archive.ForTest != "" && archive.ForTest != testVariantFor {
			return nil, fmt.Errorf("resolved archive %q targets tests for %q, want %q", importPath, archive.ForTest, testVariantFor)
		}
		if importPath == testVariantFor {
			return nil, fmt.Errorf("resolver attempted to replace authoritative package-under-test archive %q", importPath)
		}
		current, found := reg.PackageFile[importPath]
		if found && (archive.ForTest == "" || current == archive.ExportFile) {
			continue
		}
		reg.PackageFile[importPath] = archive.ExportFile
		changed[importPath] = archive.ExportFile
	}
	return changed, nil
}
