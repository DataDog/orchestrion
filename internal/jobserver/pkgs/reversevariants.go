// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package pkgs

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/DataDog/orchestrion/internal/toolexec/archive"
	"github.com/rs/zerolog"
	"golang.org/x/tools/go/packages"
)

const (
	envVarReverseVariant       = "ORCHESTRION_REVERSE_VARIANT"
	envVarReverseVariantFlavor = "ORCHESTRION_REVERSE_VARIANT_FLAVOR"
)

type reverseVariantEnvironment struct {
	Flavor       string            `json:"flavor"`
	PackageFiles map[string]string `json:"packageFiles"`
}

// ReverseVariantEnvironment returns the flavor and authoritative package files
// for a nested build that reconstructs synthetic importers for a test binary.
func ReverseVariantEnvironment() (flavor string, packageFiles map[string]string, active bool, err error) {
	path := os.Getenv(envVarReverseVariant)
	if path == "" {
		return "", nil, false, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", nil, true, fmt.Errorf("reading reverse test variant environment %q: %w", path, err)
	}
	var value reverseVariantEnvironment
	if err := json.Unmarshal(data, &value); err != nil {
		return "", nil, true, fmt.Errorf("parsing reverse test variant environment %q: %w", path, err)
	}
	if value.Flavor == "" {
		return "", nil, true, fmt.Errorf("reverse test variant environment %q has no flavor", path)
	}
	if flavor := os.Getenv(envVarReverseVariantFlavor); flavor != value.Flavor {
		return "", nil, true, fmt.Errorf("reverse test variant environment %q has flavor %q, process declares %q", path, value.Flavor, flavor)
	}
	if len(value.PackageFiles) == 0 {
		return "", nil, true, fmt.Errorf("reverse test variant environment %q has no package files", path)
	}
	return value.Flavor, value.PackageFiles, true, nil
}

func (s *service) resolveReverseTestVariant(ctx context.Context, req *ResolveRequest, log zerolog.Logger) (resolvedPackageSet, error) {
	if req.Pattern == "" || req.TestVariantFor == "" || req.AuthoritativeTarget == "" {
		return resolvedPackageSet{}, fmt.Errorf("incomplete reverse test variant request for %q", req.Pattern)
	}

	ordinaryReq := *req
	ordinaryReq.Pattern = req.TestVariantFor
	ordinaryReq.TestVariantFor = ""
	ordinaryReq.ReverseTestVariant = false
	ordinaryReq.AuthoritativeTarget = ""
	ordinaryHash, err := ordinaryReq.hash()
	if err != nil {
		return resolvedPackageSet{}, err
	}
	ordinary, err := s.resolved.Load(ordinaryHash, func() (resolvedPackageSet, error) {
		return loadResolvedPackages(ctx, &ordinaryReq, log)
	})
	if err != nil {
		return resolvedPackageSet{}, err
	}
	if ordinary.buildFlagsErr != "" {
		return resolvedPackageSet{}, fmt.Errorf("obtaining Go build flags for reverse test variant resolution: %s", ordinary.buildFlagsErr)
	}

	config := ordinary.config
	config.Context = ctx
	config.Env = resolveEnvironment(ctx, req)
	config.BuildFlags = slices.Clone(config.BuildFlags)
	if ordinary.testCoverpkgInferred {
		coveragePackages := slices.Clone(ordinary.testPackagesWithoutTests)
		if !slices.Contains(coveragePackages, req.TestVariantFor) {
			coveragePackages = append(coveragePackages, req.TestVariantFor)
		}
		config.BuildFlags = scopeInferredTestCoverage(config.BuildFlags, ordinary.testCoverageMode, coveragePackages)
	}

	return buildReverseTestVariant(req, config)
}

func buildReverseTestVariant(req *ResolveRequest, config packages.Config) (resolvedPackageSet, error) {
	// Detect same-package test augmentation before comparing the authoritative
	// archive. Reconstructing a reverse edge through that augmented package would
	// require solving a compiler-fingerprint cycle, so preserve the explicit
	// safety failure instead of reporting a misleading coverage mismatch.
	provenanceConfig := config
	provenanceConfig.Tests = true
	provenanceConfig.Mode = packages.NeedName | packages.NeedImports | packages.NeedDeps | packages.NeedForTest
	provenance, err := packages.Load(&provenanceConfig, req.TestVariantFor)
	if err != nil {
		return resolvedPackageSet{}, fmt.Errorf("loading reverse test target provenance for %q: %w", req.TestVariantFor, err)
	}
	if err := packageErrors(provenance); err != nil {
		return resolvedPackageSet{}, fmt.Errorf("loading reverse test target provenance for %q: %w", req.TestVariantFor, err)
	}
	if findTestVariant(collectPackages(provenance), req.TestVariantFor, req.TestVariantFor) != nil {
		return resolvedPackageSet{}, fmt.Errorf("synthetic importer %q cannot be rebuilt against same-package tests for %q without creating a package fingerprint cycle", req.Pattern, req.TestVariantFor)
	}

	// Resolve the package-under-test closure under the same scoped coverage flags
	// as the nested test graph. Synthetic importers receive this closure directly
	// and therefore cannot recurse into the jobserver while it is being built.
	config.Tests = false
	config.Mode = packages.NeedName | packages.NeedImports | packages.NeedDeps |
		packages.NeedExportFile | packages.NeedForTest
	targets, err := packages.Load(&config, req.TestVariantFor)
	if err != nil {
		return resolvedPackageSet{}, fmt.Errorf("loading authoritative reverse test target %q: %w", req.TestVariantFor, err)
	}
	if err := packageErrors(targets); err != nil {
		return resolvedPackageSet{}, fmt.Errorf("loading authoritative reverse test target %q: %w", req.TestVariantFor, err)
	}
	overrides := make(ResolveResponse)
	for _, target := range targets {
		if err := overrides.mergeFrom(target); err != nil {
			return resolvedPackageSet{}, err
		}
	}
	selected, found := overrides[req.TestVariantFor]
	if !found || selected.ExportFile == "" {
		return resolvedPackageSet{}, fmt.Errorf("Go did not produce an export archive for reverse test target %q", req.TestVariantFor)
	}
	resolvedFingerprint, err := archive.Fingerprint(selected.ExportFile)
	if err != nil {
		return resolvedPackageSet{}, err
	}
	authoritativeFingerprint, err := archive.Fingerprint(req.AuthoritativeTarget)
	if err != nil {
		return resolvedPackageSet{}, err
	}
	if resolvedFingerprint != authoritativeFingerprint {
		return resolvedPackageSet{}, fmt.Errorf("scoped reverse test target %q has fingerprint %s, outer test binary selected %s", req.TestVariantFor, resolvedFingerprint, authoritativeFingerprint)
	}
	selected.ExportFile = req.AuthoritativeTarget
	overrides[req.TestVariantFor] = selected

	packageFiles := make(map[string]string, len(overrides))
	paths := make([]string, 0, len(overrides))
	for path, resolved := range overrides {
		packageFiles[path] = resolved.ExportFile
		paths = append(paths, path)
	}
	slices.Sort(paths)
	contractHash := sha256.New()
	for _, path := range paths {
		fingerprint, err := archive.CompatibilityFingerprint(packageFiles[path])
		if err != nil {
			return resolvedPackageSet{}, fmt.Errorf("fingerprinting authoritative reverse test dependency %q: %w", path, err)
		}
		_, _ = contractHash.Write([]byte(path))
		_, _ = contractHash.Write([]byte{0})
		_, _ = contractHash.Write([]byte(fingerprint))
		_, _ = contractHash.Write([]byte{0})
	}
	// The target path and the complete injected closure identify the compiler
	// contract consumed out-of-band by synthetic importers. Using that contract
	// as the tool flavor isolates incompatible coverage scopes while allowing
	// compatible reverse variants to reuse Go's cache across runs.
	targetHash := sha256.Sum256([]byte(req.TestVariantFor))
	flavor := hex.EncodeToString(targetHash[:8]) + "-" + hex.EncodeToString(contractHash.Sum(nil)[:16])
	config.Env = append(config.Env, envVarResolvingTestVariants+"=1")
	cleanup, err := addReverseVariantEnvironment(&config, req.TempDir, flavor, packageFiles)
	if err != nil {
		return resolvedPackageSet{}, err
	}
	defer cleanup()

	config.Tests = true
	config.Mode = packages.NeedName | packages.NeedCompiledGoFiles | packages.NeedImports |
		packages.NeedDeps | packages.NeedExportFile | packages.NeedForTest
	loaded, err := packages.Load(&config, req.TestVariantFor)
	if err != nil {
		return resolvedPackageSet{}, fmt.Errorf("loading reverse test variants for %q: %w", req.TestVariantFor, err)
	}
	if err := packageErrors(loaded); err != nil {
		return resolvedPackageSet{}, fmt.Errorf("constructing reverse test variant for synthetic importer %q of %q: %w", req.Pattern, req.TestVariantFor, err)
	}
	all := collectPackages(loaded)
	root := findTestVariant(all, req.Pattern, req.TestVariantFor)
	if root == nil {
		// The synthetic importer may itself be a link dependency that is absent
		// from the package-under-test graph. Build that root in the same flavored
		// universe; the generated test main will import the rebuilt root directly.
		config.Tests = false
		standalone, err := packages.Load(&config, req.Pattern)
		if err != nil {
			return resolvedPackageSet{}, fmt.Errorf("loading standalone reverse test variant %q for %q: %w", req.Pattern, req.TestVariantFor, err)
		}
		if err := packageErrors(standalone); err != nil {
			return resolvedPackageSet{}, fmt.Errorf("constructing standalone reverse test variant %q for %q: %w", req.Pattern, req.TestVariantFor, err)
		}
		root = findPackage(collectPackages(standalone), req.Pattern)
		if root == nil {
			return resolvedPackageSet{}, fmt.Errorf("Go did not produce reverse test variant %q for %q", req.Pattern, req.TestVariantFor)
		}
	}

	resp := make(ResolveResponse)
	if err := collectReverseVariantClosure(config.Context, resp, root, req.TestVariantFor, make(map[string]bool)); err != nil {
		return resolvedPackageSet{}, err
	}
	delete(resp, req.TestVariantFor)
	return resolvedPackageSet{response: resp}, nil
}

func collectReverseVariantClosure(ctx context.Context, resp ResolveResponse, pkg *packages.Package, target string, visited map[string]bool) error {
	if pkg == nil || visited[pkg.ID] || pkg.PkgPath == target {
		return nil
	}
	visited[pkg.ID] = true
	if pkg.PkgPath != "" && pkg.PkgPath != "unsafe" {
		if pkg.ExportFile == "" {
			return fmt.Errorf("Go did not produce an export archive for reverse test variant %q (%s) of %q", pkg.PkgPath, pkg.ID, target)
		}
		resp[pkg.PkgPath] = ResolvedArchive{ExportFile: pkg.ExportFile, ForTest: target}
	}
	for _, imported := range pkg.Imports {
		if err := collectReverseVariantClosure(ctx, resp, imported, target, visited); err != nil {
			return err
		}
	}
	return nil
}

func addReverseVariantEnvironment(config *packages.Config, tempDir string, flavor string, packageFiles map[string]string) (func(), error) {
	dir, err := os.MkdirTemp(tempDir, "orchestrion-reverse-variant-")
	if err != nil {
		return nil, fmt.Errorf("creating reverse test variant environment: %w", err)
	}
	cleanup := func() { _ = os.RemoveAll(dir) }
	path := filepath.Join(dir, "environment.json")
	file, err := os.Create(path)
	if err != nil {
		cleanup()
		return nil, fmt.Errorf("creating reverse test variant environment: %w", err)
	}
	value := reverseVariantEnvironment{Flavor: flavor, PackageFiles: packageFiles}
	if err := json.NewEncoder(file).Encode(value); err != nil {
		_ = file.Close()
		cleanup()
		return nil, fmt.Errorf("writing reverse test variant environment: %w", err)
	}
	if err := file.Close(); err != nil {
		cleanup()
		return nil, fmt.Errorf("closing reverse test variant environment: %w", err)
	}
	config.Env = append(config.Env,
		envVarReverseVariant+"="+path,
		envVarReverseVariantFlavor+"="+flavor,
	)
	return cleanup, nil
}
