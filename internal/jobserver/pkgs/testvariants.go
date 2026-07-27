// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package pkgs

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
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

	"github.com/DataDog/dd-trace-go/v2/ddtrace/tracer"
	"github.com/DataDog/orchestrion/internal/binpath"
	"github.com/DataDog/orchestrion/internal/goflags"
	"github.com/DataDog/orchestrion/internal/jobserver/client"
	"github.com/DataDog/orchestrion/internal/traceutil"
	"github.com/rs/zerolog"
	"golang.org/x/tools/go/packages"
)

const envVarResolvingTestVariants = "ORCHESTRION_RESOLVING_TEST_VARIANTS"

type (
	// ResolveTestVariantsRequest asks the jobserver to construct the Go test copies of synthetic
	// dependencies that transitively import PackageUnderTest.
	ResolveTestVariantsRequest struct {
		Dir              string   `json:"dir"`
		Env              []string `json:"env"`
		TempDir          string   `json:"tmpdir,omitempty"`
		PackageUnderTest string   `json:"packageUnderTest"`
		SyntheticRoots   []string `json:"syntheticRoots"`

		resolveParentID    string
		toolexecImportpath string
		canonical          bool
	}

	// ResolveTestVariantsResponse maps import paths to export archives built for the requested test.
	ResolveTestVariantsResponse map[string]string
)

// NewResolveTestVariantsRequest creates a request using the current process environment.
func NewResolveTestVariantsRequest(dir string, packageUnderTest string, syntheticRoots []string) *ResolveTestVariantsRequest {
	return &ResolveTestVariantsRequest{
		Dir:              dir,
		Env:              os.Environ(),
		PackageUnderTest: packageUnderTest,
		SyntheticRoots:   slices.Clone(syntheticRoots),
	}
}

func (ResolveTestVariantsRequest) Subject() string                        { return resolveTestVariantsSubject }
func (ResolveTestVariantsRequest) ResponseIs(ResolveTestVariantsResponse) {}
func (r ResolveTestVariantsRequest) ForeachSpanTag(set func(key string, value any)) {
	set("request.dir", r.Dir)
	set("request.package-under-test", r.PackageUnderTest)
	set("request.synthetic-roots", r.SyntheticRoots)
}

// ResolvingTestVariants reports whether the current process belongs to the nested test load used
// to construct synthetic dependency variants.
func ResolvingTestVariants() bool {
	return os.Getenv(envVarResolvingTestVariants) != ""
}

func (r *ResolveTestVariantsRequest) canonicalize() {
	if r.canonical {
		return
	}
	r.Env, r.resolveParentID, r.toolexecImportpath = canonicalizeEnviron(r.Env, &r.TempDir)
	slices.Sort(r.SyntheticRoots)
	r.SyntheticRoots = slices.Compact(r.SyntheticRoots)
	r.canonical = true
}

func (r *ResolveTestVariantsRequest) hash() (string, error) {
	r.canonicalize()
	hash := sha512.New()
	if err := json.NewEncoder(hash).Encode(r); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(hash.Sum(nil)), nil
}

func (s *service) resolveTestVariants(ctx context.Context, req *ResolveTestVariantsRequest) (ResolveTestVariantsResponse, error) {
	log := zerolog.Ctx(ctx)

	req.Env = append(req.Env, fmt.Sprintf("%s=%s", client.EnvVarJobserverURL, s.serverURL))
	req.canonicalize()
	if req.PackageUnderTest == "" {
		return nil, errors.New("test variant resolution requires a package under test")
	}
	if len(req.SyntheticRoots) == 0 {
		return make(ResolveTestVariantsResponse), nil
	}

	reqHash, err := req.hash()
	if err != nil {
		return nil, err
	}
	if req.resolveParentID != "" {
		if err := s.graph.AddEdge(req.resolveParentID, req.toolexecImportpath); err != nil {
			return nil, err
		}
		defer s.graph.RemoveEdge(req.resolveParentID, req.toolexecImportpath)
	}

	return s.testVariants.Load(reqHash, func() (_ ResolveTestVariantsResponse, err error) {
		span, ctx := tracer.StartSpanFromContext(ctx, "pkgs.ResolveTestVariants")
		defer func() { span.Finish(tracer.WithError(err)) }()

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

		patterns := append(slices.Clone(req.SyntheticRoots), req.PackageUnderTest)
		ordinary, err := packages.Load(&packages.Config{
			Context: ctx,
			Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles |
				packages.NeedImports | packages.NeedDeps | packages.NeedExportFile,
			Dir: req.Dir, Env: env, BuildFlags: buildFlags, Logf: loadLogf,
		}, patterns...)
		if err != nil {
			return nil, fmt.Errorf("loading synthetic dependency graph: %w", err)
		}
		if err := packageErrors(ordinary); err != nil {
			return nil, fmt.Errorf("loading synthetic dependency graph: %w", err)
		}

		var packageUnderTest *packages.Package
		roots := make(map[string]*packages.Package, len(req.SyntheticRoots))
		for _, pkg := range ordinary {
			if pkg.PkgPath == req.PackageUnderTest {
				packageUnderTest = pkg
			}
			if slices.Contains(req.SyntheticRoots, pkg.PkgPath) {
				roots[pkg.PkgPath] = pkg
			}
		}
		if packageUnderTest == nil || len(packageUnderTest.GoFiles) == 0 {
			return nil, fmt.Errorf("package under test %q has no source directory", req.PackageUnderTest)
		}

		affected := make([]string, 0, len(req.SyntheticRoots))
		for _, root := range req.SyntheticRoots {
			pkg := roots[root]
			if pkg == nil {
				return nil, fmt.Errorf("synthetic dependency graph did not include root %q", root)
			}
			if importsPackage(pkg, req.PackageUnderTest, make(map[string]bool)) {
				affected = append(affected, root)
			}
		}
		if len(affected) == 0 {
			return make(ResolveTestVariantsResponse), nil
		}

		overlayKey := sha256.Sum256([]byte(req.PackageUnderTest + "\x00" + strings.Join(affected, "\x00")))
		overlayPath := filepath.Join(
			filepath.Dir(packageUnderTest.GoFiles[0]),
			fmt.Sprintf("zz_orchestrion_linkdeps_%x_test.go", overlayKey[:8]),
		)
		if _, err := os.Stat(overlayPath); err == nil {
			return nil, fmt.Errorf("refusing to replace existing source file %q with a test variant overlay", overlayPath)
		} else if !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("checking test variant overlay path %q: %w", overlayPath, err)
		}
		overlaySource, err := testVariantOverlay(packageUnderTest.Name, affected)
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
		}, req.PackageUnderTest)
		if err != nil {
			return nil, fmt.Errorf("loading test variants for %q: %w", req.PackageUnderTest, err)
		}
		if err := packageErrors(loaded); err != nil {
			return nil, fmt.Errorf(
				"constructing test variants for synthetic dependencies %q that import %q: %w; ordinary archives cannot be used because they expect a different package fingerprint",
				affected,
				req.PackageUnderTest,
				err,
			)
		}

		all := collectPackages(loaded)
		if findTestVariant(all, req.PackageUnderTest, req.PackageUnderTest) == nil {
			// With external tests only, cmd/go does not augment the package under test and
			// therefore has no fingerprints that synthetic dependencies must be rebuilt against.
			return make(ResolveTestVariantsResponse), nil
		}
		resp := make(ResolveTestVariantsResponse)
		for _, root := range affected {
			variant := findTestVariant(all, root, req.PackageUnderTest)
			if variant == nil || variant.ExportFile == "" {
				return nil, fmt.Errorf("Go did not produce a test variant for synthetic dependency %q of %q", root, req.PackageUnderTest)
			}
			collectTestVariantClosure(resp, variant, req.PackageUnderTest, make(map[string]bool))
		}
		delete(resp, req.PackageUnderTest)
		return resp, nil
	})
}

func (_ *service) testVariantEnvironment(req *ResolveTestVariantsRequest, span *tracer.Span) ([]string, error) {
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

func testVariantOverlay(packageName string, roots []string) ([]byte, error) {
	imports := slices.Clone(roots)
	slices.Sort(imports)
	imports = slices.Compact(imports)
	decl := &ast.GenDecl{Tok: token.IMPORT, Specs: make([]ast.Spec, len(imports))}
	file := &ast.File{Name: ast.NewIdent(packageName + "_test"), Decls: []ast.Decl{decl}}
	for i, path := range imports {
		decl.Specs[i] = &ast.ImportSpec{Name: ast.NewIdent("_"), Path: &ast.BasicLit{Kind: token.STRING, Value: strconv.Quote(path)}}
	}
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

func collectTestVariantClosure(resp ResolveTestVariantsResponse, pkg *packages.Package, forTest string, visited map[string]bool) {
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
