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
		switch pkg.PkgPath {
		case req.TestVariantFor:
			packageUnderTest = pkg
		case req.Pattern:
			root = pkg
		}
	}
	if root == nil {
		return nil, fmt.Errorf("synthetic dependency graph did not include root %q", req.Pattern)
	}
	if !importsPackage(root, req.TestVariantFor, make(map[string]bool)) {
		return resp, nil
	}
	var srcFile string
	if packageUnderTest != nil {
		if len(packageUnderTest.GoFiles) > 0 {
			srcFile = packageUnderTest.GoFiles[0]
		} else if len(packageUnderTest.CompiledGoFiles) > 0 {
			srcFile = packageUnderTest.CompiledGoFiles[0]
		} else if len(packageUnderTest.CgoFiles) > 0 {
			srcFile = packageUnderTest.CgoFiles[0]
		}
	}
	if srcFile == "" {
		return nil, fmt.Errorf("package under test %q has no files to locate its source directory", req.TestVariantFor)
	}

	overlayKey := sha256.Sum256([]byte(req.TestVariantFor + "\x00" + req.Pattern))
	overlayPath := filepath.Join(
		filepath.Dir(srcFile),
		fmt.Sprintf("zz_orchestrion_linkdeps_%x_test.go", overlayKey[:8]),
	)
	if _, err := os.Stat(overlayPath); err == nil {
		return nil, fmt.Errorf("refusing to replace existing source file %q with a test variant overlay", overlayPath)
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("checking test variant overlay path %q: %w", overlayPath, err)
	}
	overlaySource, err := testVariantOverlay(packageUnderTest.Name, req.Pattern)
	if err != nil {
		return nil, err
	}
	config.Context = ctx
	config.Mode = packages.NeedName | packages.NeedCompiledGoFiles | packages.NeedImports |
		packages.NeedDeps | packages.NeedExportFile | packages.NeedForTest
	config.Env = append(slices.Clone(config.Env), envVarResolvingTestVariants+"=1")
	config.Tests = true
	config.Overlay = map[string][]byte{overlayPath: overlaySource}
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
	collectTestVariantClosure(resp, variant, req.TestVariantFor, make(map[string]bool))
	delete(resp, req.TestVariantFor)
	return resp, nil
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

func testVariantOverlay(packageName string, root string) ([]byte, error) {
	decl := &ast.GenDecl{Tok: token.IMPORT, Specs: []ast.Spec{
		&ast.ImportSpec{Name: ast.NewIdent("_"), Path: &ast.BasicLit{Kind: token.STRING, Value: strconv.Quote(root)}},
	}}
	file := &ast.File{Name: ast.NewIdent(packageName + "_test"), Decls: []ast.Decl{decl}}
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

func collectTestVariantClosure(resp ResolveResponse, pkg *packages.Package, forTest string, visited map[string]bool) {
	if pkg == nil || visited[pkg.ID] {
		return
	}
	visited[pkg.ID] = true
	if pkg.ForTest != forTest {
		return
	}
	if pkg.PkgPath != "" && pkg.PkgPath != "unsafe" && pkg.ExportFile != "" {
		resp[pkg.PkgPath] = pkg.ExportFile
	}
	for _, imported := range pkg.Imports {
		collectTestVariantClosure(resp, imported, forTest, visited)
	}
}
