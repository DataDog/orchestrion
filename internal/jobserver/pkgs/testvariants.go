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

	"github.com/DataDog/dd-trace-go/v2/ddtrace/tracer"
	"github.com/DataDog/orchestrion/internal/binpath"
	"github.com/DataDog/orchestrion/internal/goflags"
	"github.com/DataDog/orchestrion/internal/traceutil"
	"github.com/rs/zerolog"
	"golang.org/x/tools/go/packages"
)

const envVarResolvingTestVariants = "ORCHESTRION_RESOLVING_TEST_VARIANTS"

// ResolvingTestVariants reports whether the current process belongs to the nested test load used
// to construct synthetic dependency variants.
func ResolvingTestVariants() bool {
	return os.Getenv(envVarResolvingTestVariants) != ""
}

func (s *service) resolveTestVariant(ctx context.Context, req *ResolveRequest) (_ ResolveResponse, err error) {
	log := zerolog.Ctx(ctx)
	span, ctx := tracer.StartSpanFromContext(ctx, "pkgs.ResolveTestVariant")
	defer func() { span.Finish(tracer.WithError(err)) }()

	if req.Pattern == "" {
		return nil, errors.New("test variant resolution requires a package pattern")
	}

	env, err := s.testVariantEnvironment(req, span)
	if err != nil {
		return nil, err
	}
	buildFlags, err := testVariantBuildFlags(ctx)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to obtain go build flags")
	}
	loadLogf := func(format string, args ...any) {
		log.Trace().Str("operation", "packages.Load").Msgf(format, args...)
	}

	ordinary, err := packages.Load(&packages.Config{
		Context: ctx,
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles |
			packages.NeedImports | packages.NeedDeps | packages.NeedExportFile,
		Dir: req.Dir, Env: env, BuildFlags: buildFlags, Logf: loadLogf,
	}, req.Pattern)
	if err != nil {
		return nil, fmt.Errorf("loading synthetic dependency graph: %w", err)
	}
	if err := packageErrors(ordinary); err != nil {
		return nil, fmt.Errorf("loading synthetic dependency graph: %w", err)
	}

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

	resp := make(ResolveResponse)
	if err := resp.mergeFrom(root); err != nil {
		return nil, fmt.Errorf("collecting ordinary synthetic dependency graph: %w", err)
	}
	if !importsPackage(root, req.TestVariantFor, make(map[string]bool)) {
		return resp, nil
	}
	if packageUnderTest == nil || len(packageUnderTest.GoFiles) == 0 {
		return nil, fmt.Errorf("package under test %q has no source directory", req.TestVariantFor)
	}

	overlayKey := sha256.Sum256([]byte(req.TestVariantFor + "\x00" + req.Pattern))
	overlayPath := filepath.Join(
		filepath.Dir(packageUnderTest.GoFiles[0]),
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
	loaded, err := packages.Load(&packages.Config{
		Context: ctx,
		Mode: packages.NeedName | packages.NeedCompiledGoFiles | packages.NeedImports |
			packages.NeedDeps | packages.NeedExportFile | packages.NeedForTest,
		Dir: req.Dir, Env: env, BuildFlags: buildFlags, Logf: loadLogf,
		Tests:   true,
		Overlay: map[string][]byte{overlayPath: overlaySource},
	}, req.TestVariantFor)
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

func (_ *service) testVariantEnvironment(req *ResolveRequest, span *tracer.Span) ([]string, error) {
	env := slices.Clone(req.Env)
	tracer.Inject(span.Context(), traceutil.EnvVarCarrier{Env: &env})
	if req.toolexecImportpath != "" {
		env = append(env, fmt.Sprintf("%s=%s", envVarParentID, req.toolexecImportpath))
	}
	if req.TempDir != "" {
		if err := os.MkdirAll(req.TempDir, 0o755); err != nil {
			return nil, fmt.Errorf("creating temporary directory %q: %w", req.TempDir, err)
		}
		env = append(env, fmt.Sprintf("%s=%s", envVarGotmpdir, req.TempDir))
	}
	env = append(env, envVarResolvingTestVariants+"=1")
	return env, nil
}

func testVariantBuildFlags(ctx context.Context) ([]string, error) {
	flags, err := goflags.Flags(ctx)
	flags = flags.Except("-a", "-toolexec")
	return append(flags.Slice(), fmt.Sprintf("-toolexec=%q toolexec", binpath.Orchestrion)), err
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
