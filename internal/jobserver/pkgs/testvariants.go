// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package pkgs

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"go/ast"
	"go/format"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"golang.org/x/tools/go/packages"
)

const envVarResolvingTestVariants = "ORCHESTRION_RESOLVING_TEST_VARIANTS"

// ResolvingTestVariants reports whether the current process belongs to the nested test load used
// to construct synthetic dependency variants.
func ResolvingTestVariants() bool {
	return os.Getenv(envVarResolvingTestVariants) != ""
}

func mergeTestVariant(
	ctx context.Context,
	req *ResolveRequest,
	ordinary []*packages.Package,
	resp ResolveResponse,
	config packages.Config,
) (ResolveResponse, error) {
	var packageUnderTest, root *packages.Package
	for _, pkg := range collectPackages(ordinary) {
		if pkg.PkgPath == req.TestVariantFor {
			packageUnderTest = pkg
		}
		if pkg.PkgPath == req.Pattern {
			root = pkg
		}
	}
	if root == nil {
		return nil, fmt.Errorf("synthetic dependency graph did not include root %q", req.Pattern)
	}
	if req.Pattern == req.TestVariantFor {
		return resolveTestTargetProvenance(ctx, req, resp, config)
	}
	if !importsPackage(root, req.TestVariantFor, make(map[string]bool)) {
		return resp, nil
	}
	sourceDir := packageSourceDir(packageUnderTest)
	if sourceDir == "" {
		return nil, fmt.Errorf("package under test %q has no source directory", req.TestVariantFor)
	}

	overlayKey := sha256.Sum256([]byte(req.TestVariantFor + "\x00" + req.Pattern))
	overlayPath := filepath.Join(
		sourceDir,
		fmt.Sprintf("zz_orchestrion_linkdeps_%x_test.go", overlayKey[:8]),
	)
	if _, err := os.Stat(overlayPath); err == nil {
		return nil, fmt.Errorf("refusing to replace existing source file %q with a test variant overlay", overlayPath)
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("checking test variant overlay path %q: %w", overlayPath, err)
	}
	overlays := make(map[string]overlaySource)
	testImportPath := req.Pattern
	if bridge, needed, err := internalImportBridge(root, req.TestVariantFor, overlayKey); err != nil {
		return nil, err
	} else if needed {
		testImportPath = bridge.importPath
		overlays[bridge.virtualPath] = overlaySource{contents: bridge.source, newPackage: true}
	}
	testSource, err := testVariantOverlay(packageUnderTest.Name, testImportPath)
	if err != nil {
		return nil, err
	}
	overlays[overlayPath] = overlaySource{contents: testSource}
	config.Context = ctx
	config.Mode = packages.NeedName | packages.NeedCompiledGoFiles | packages.NeedImports |
		packages.NeedDeps | packages.NeedExportFile | packages.NeedForTest
	config.Env = append(slices.Clone(config.Env), envVarResolvingTestVariants+"=1")
	config.Tests = true
	cleanup, err := addTestVariantOverlays(ctx, &config, overlays)
	if err != nil {
		return nil, err
	}
	defer cleanup()
	loaded, err := packages.Load(&config, req.TestVariantFor)
	if err != nil {
		return nil, fmt.Errorf("loading test variants for %q: %w", req.TestVariantFor, err)
	}
	if err := packageErrors(loaded); err != nil {
		return nil, fmt.Errorf(
			"constructing a test variant for synthetic dependency %q that imports %q: %w; the ordinary archive cannot be used because it expects a different package fingerprint",
			req.Pattern,
			req.TestVariantFor,
			err,
		)
	}

	all := collectPackages(loaded)
	if findTestVariant(all, req.TestVariantFor, req.TestVariantFor) == nil {
		// With external tests only, cmd/go does not augment the package under test and
		// therefore has no fingerprints that synthetic dependencies must be rebuilt against.
		delete(resp, req.TestVariantFor)
		return resp, nil
	}
	variant := findTestVariant(all, req.Pattern, req.TestVariantFor)
	if variant == nil || variant.ExportFile == "" {
		return nil, fmt.Errorf("Go did not produce a test variant for synthetic dependency %q of %q", req.Pattern, req.TestVariantFor)
	}
	if err := collectTestVariantClosure(resp, variant, req.TestVariantFor, make(map[string]bool)); err != nil {
		return nil, err
	}
	delete(resp, req.TestVariantFor)
	return resp, nil
}

// resolveTestTargetProvenance returns the package-under-test archive solely so callers can
// determine whether Go augmented it with same-package tests. This response must not be merged
// into the outer importcfg, whose package-under-test archive remains authoritative.
func resolveTestTargetProvenance(ctx context.Context, req *ResolveRequest, resp ResolveResponse, config packages.Config) (ResolveResponse, error) {
	config.Context = ctx
	config.Mode = packages.NeedName | packages.NeedImports | packages.NeedDeps | packages.NeedForTest
	config.Env = append(slices.Clone(config.Env), envVarResolvingTestVariants+"=1")
	config.Tests = true
	loaded, err := packages.Load(&config, req.TestVariantFor)
	if err != nil {
		return nil, fmt.Errorf("loading test target provenance for %q: %w", req.TestVariantFor, err)
	}
	if err := packageErrors(loaded); err != nil {
		return nil, fmt.Errorf("loading test target provenance for %q: %w", req.TestVariantFor, err)
	}
	variant := findTestVariant(collectPackages(loaded), req.TestVariantFor, req.TestVariantFor)
	if variant == nil {
		return resp, nil
	}
	selected := resp[req.TestVariantFor]
	selected.ForTest = req.TestVariantFor
	resp[req.TestVariantFor] = selected
	return resp, nil
}

func packageSourceDir(pkg *packages.Package) string {
	if pkg == nil {
		return ""
	}
	for _, files := range [][]string{pkg.GoFiles, pkg.OtherFiles} {
		if len(files) > 0 {
			return filepath.Dir(files[0])
		}
	}
	return ""
}

func importsPackage(pkg *packages.Package, target string, visited map[string]bool) bool {
	if pkg == nil || visited[pkg.ID] {
		return false
	}
	visited[pkg.ID] = true
	for _, imported := range pkg.Imports {
		if imported.PkgPath == target || importsPackage(imported, target, visited) {
			return true
		}
	}
	return false
}

type importBridge struct {
	importPath  string
	virtualPath string
	source      []byte
}

func internalImportBridge(root *packages.Package, importer string, key [sha256.Size]byte) (importBridge, bool, error) {
	segments := strings.Split(root.PkgPath, "/")
	var restrictedPrefix string
	var restricted bool
	for i, segment := range segments {
		if segment != "internal" {
			continue
		}
		prefix := strings.Join(segments[:i], "/")
		if importer == prefix || strings.HasPrefix(importer, prefix+"/") {
			continue
		}
		if restricted {
			return importBridge{}, false, fmt.Errorf("cannot construct test variant for nested internal synthetic dependency %q imported by %q", root.PkgPath, importer)
		}
		restrictedPrefix = prefix
		restricted = true
	}
	if !restricted {
		return importBridge{}, false, nil
	}

	if restrictedPrefix == "" {
		return importBridge{}, false, fmt.Errorf("cannot construct test variant for top-level internal synthetic dependency %q imported by %q", root.PkgPath, importer)
	}
	rootDir := packageSourceDir(root)
	if rootDir == "" {
		return importBridge{}, false, fmt.Errorf("synthetic dependency %q has no source directory", root.PkgPath)
	}
	prefixSegments := strings.Count(restrictedPrefix, "/") + 1
	for range len(segments) - prefixSegments {
		rootDir = filepath.Dir(rootDir)
	}
	name := fmt.Sprintf("orchestrion_test_variant_%x", key[:8])
	importPath := restrictedPrefix + "/" + name
	virtualPath := filepath.Join(rootDir, name, "bridge.go")
	if _, err := os.Stat(virtualPath); err == nil {
		return importBridge{}, false, fmt.Errorf("refusing to replace existing source file %q with a test variant bridge", virtualPath)
	} else if !errors.Is(err, os.ErrNotExist) {
		return importBridge{}, false, fmt.Errorf("checking test variant bridge path %q: %w", virtualPath, err)
	}
	source, err := blankImportSource("orchestrion_test_variant_bridge", root.PkgPath)
	if err != nil {
		return importBridge{}, false, err
	}
	return importBridge{importPath: importPath, virtualPath: virtualPath, source: source}, true, nil
}

func testVariantOverlay(packageName string, root string) ([]byte, error) {
	return blankImportSource(packageName+"_test", root)
}

func blankImportSource(packageName string, imported string) ([]byte, error) {
	decl := &ast.GenDecl{Tok: token.IMPORT, Specs: []ast.Spec{
		&ast.ImportSpec{Name: ast.NewIdent("_"), Path: &ast.BasicLit{Kind: token.STRING, Value: strconv.Quote(imported)}},
	}}
	file := &ast.File{Name: ast.NewIdent(packageName), Decls: []ast.Decl{decl}}
	var source bytes.Buffer
	if err := format.Node(&source, token.NewFileSet(), file); err != nil {
		return nil, fmt.Errorf("formatting test variant overlay: %w", err)
	}
	return source.Bytes(), nil
}

func packageErrors(roots []*packages.Package) error {
	var result error
	for _, pkg := range collectPackages(roots) {
		for _, pkgErr := range pkg.Errors {
			result = errors.Join(result, pkgErr)
		}
	}
	return result
}

func collectPackages(roots []*packages.Package) []*packages.Package {
	var result []*packages.Package
	visited := make(map[string]bool)
	var visit func(*packages.Package)
	visit = func(pkg *packages.Package) {
		if pkg == nil || visited[pkg.ID] {
			return
		}
		visited[pkg.ID] = true
		result = append(result, pkg)
		for _, imported := range pkg.Imports {
			visit(imported)
		}
	}
	for _, root := range roots {
		visit(root)
	}
	return result
}

func findTestVariant(pkgs []*packages.Package, pkgPath string, forTest string) *packages.Package {
	for _, pkg := range pkgs {
		if pkg.PkgPath == pkgPath && pkg.ForTest == forTest {
			return pkg
		}
	}
	return nil
}

func collectTestVariantClosure(resp ResolveResponse, pkg *packages.Package, forTest string, visited map[string]bool) error {
	if pkg == nil || visited[pkg.ID] {
		return nil
	}
	visited[pkg.ID] = true
	if pkg.ForTest != forTest {
		return nil
	}
	if pkg.PkgPath != "" && pkg.PkgPath != "unsafe" {
		if pkg.ExportFile == "" {
			return fmt.Errorf("Go did not produce an export archive for test variant %q (%s) of %q", pkg.PkgPath, pkg.ID, forTest)
		}
		resp[pkg.PkgPath] = ResolvedArchive{ExportFile: pkg.ExportFile, ForTest: forTest}
	}
	for _, imported := range pkg.Imports {
		if err := collectTestVariantClosure(resp, imported, forTest, visited); err != nil {
			return err
		}
	}
	return nil
}
