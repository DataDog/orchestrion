// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

// Package contexttest builds real type-checked, decorated ASTs together with the
// [context.AdviceContext] values that the injector would create for them.
//
// Join points and advice that rely on [context.AspectContext.ResolveType] must
// be tested against real type information. Mock contexts return nil from
// ResolveType, which makes type-guarded behavior pass vacuously. This package
// parses, type-checks, and decorates source files exactly as
// [github.com/DataDog/orchestrion/internal/injector] does, so tests can evaluate
// join points and advice against genuine [go/types] data.
//
// This package is test-only support. It must never be imported by production
// code.
package contexttest

import (
	"bytes"
	gocontext "context"
	"fmt"
	"go/ast"
	"go/format"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/DataDog/orchestrion/internal/injector/aspect/context"
	"github.com/DataDog/orchestrion/internal/injector/typed"
	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
	"github.com/dave/dst/decorator/resolver/gotypes"
	"github.com/dave/dst/decorator/resolver/simple"
	"github.com/dave/dst/dstutil"
	"github.com/stretchr/testify/require"
)

// Fixture is a type-checked, decorated source file, along with everything
// needed to build real aspect contexts for its nodes.
type Fixture struct {
	ImportPath string
	Info       *types.Info
	Importer   types.Importer
	Decorator  *decorator.Decorator
	File       *dst.File
	RefMap     *typed.ReferenceMap
	MinGoLang  context.GoLangVersion

	astFile *ast.File
}

// New parses, type-checks and decorates the provided source. The source's
// package clause determines the package name; importPath must be the import
// path of that package.
//
// extraSources maps an import path to the source of a synthetic external
// package. Those packages are fully type-checked from source (no mocks), and
// become importable from the main source. This lets tests exercise qualified
// references to packages that are not available as real dependencies.
func New(t *testing.T, importPath string, src string, extraSources map[string]string) *Fixture {
	t.Helper()

	fset := token.NewFileSet()
	imp := &testImporter{
		fallback: importer.ForCompiler(fset, "source", nil),
		packages: make(map[string]*types.Package, len(extraSources)),
	}

	for path, source := range extraSources {
		imp.packages[path] = typeCheck(t, fset, imp, path, source).Package
	}

	pkg := typeCheck(t, fset, imp, importPath, src)

	dec := decorator.NewDecoratorWithImports(fset, importPath, gotypes.New(pkg.Uses))
	dstFile, err := dec.DecorateFile(pkg.AstFile)
	require.NoError(t, err, "failed to decorate the source file")

	refMap := typed.NewReferenceMap(dec.Ast.Nodes, pkg.Scopes)

	return &Fixture{
		ImportPath: importPath,
		Info:       pkg.Info,
		Importer:   imp,
		Decorator:  dec,
		File:       dstFile,
		RefMap:     &refMap,
		astFile:    pkg.AstFile,
	}
}

// Walk traverses the decorated file and invokes fn with a real
// [context.AspectContext] for every node, mirroring how the injector builds
// contexts during injection. The walk aborts if fn returns an error.
func (f *Fixture) Walk(fn func(context.AspectContext) error) error {
	return f.walkAdvice(func(ctx context.AdviceContext) error { return fn(ctx) })
}

// WalkAdvice traverses the decorated file and invokes fn with a real
// [context.AdviceContext] for every node, so tests can apply advice directly.
// The walk aborts if fn returns an error.
func (f *Fixture) WalkAdvice(fn func(context.AdviceContext) error) error {
	return f.walkAdvice(fn)
}

func (f *Fixture) walkAdvice(fn func(context.AdviceContext) error) error {
	// The root chain is nil, exactly as in the injector's traversal.
	var chain *context.NodeChain

	var err error
	dstutil.Apply(
		f.File,
		func(csor *dstutil.Cursor) bool {
			chain = chain.Child(csor)
			return true
		},
		func(csor *dstutil.Cursor) bool {
			if err != nil {
				return false
			}

			ctx := chain.Context(gocontext.Background(), context.ContextArgs{
				Cursor:       csor,
				ImportPath:   f.ImportPath,
				File:         f.File,
				RefMap:       f.RefMap,
				SourceParser: f.Decorator,
				MinGoLang:    &f.MinGoLang,
				TypeInfo:     *f.Info,
				NodeMap:      f.Decorator.Ast.Nodes,
				Importer:     f.Importer,
			})

			err = fn(ctx)
			ctx.Release()

			old := chain
			chain = chain.Parent()
			old.Release()

			return err == nil
		},
	)

	return err
}

// callExpressions returns every *dst.CallExpr node in the file, in source
// order.
func (f *Fixture) callExpressions() []*dst.CallExpr {
	var calls []*dst.CallExpr
	_ = f.WalkAdvice(func(ctx context.AdviceContext) error {
		if call, ok := ctx.Node().(*dst.CallExpr); ok {
			calls = append(calls, call)
		}
		return nil
	})
	return calls
}

// FindCall returns the single call expression whose callee renders as callee.
// It fails the test if no such call, or more than one, exists.
func (f *Fixture) FindCall(t *testing.T, callee string) *dst.CallExpr {
	t.Helper()

	var found []*dst.CallExpr
	for _, call := range f.callExpressions() {
		if RenderExpr(call.Fun) == callee {
			found = append(found, call)
		}
	}
	require.Len(t, found, 1, "expected exactly one call to %q", callee)
	return found[0]
}

// Source renders the (possibly modified) decorated file back to Go source,
// restoring qualified identifiers through the file's own import declarations,
// as the injector does when it writes a modified file.
func (f *Fixture) Source(t *testing.T) string {
	t.Helper()

	names := make(map[string]string, len(f.astFile.Imports))
	for _, imp := range f.astFile.Imports {
		path, err := strconv.Unquote(imp.Path.Value)
		require.NoError(t, err, "failed to unquote import path %q", imp.Path.Value)
		name := ""
		if imp.Name != nil {
			name = imp.Name.Name
		}
		if name == "" || name == "." || name == "_" {
			name = path[strings.LastIndex(path, "/")+1:]
		}
		names[path] = name
	}

	restorer := decorator.NewRestorerWithImports(f.ImportPath, simple.New(names))
	astFile, err := restorer.RestoreFile(f.File)
	require.NoError(t, err, "failed to restore the decorated file")

	var buf bytes.Buffer
	require.NoError(t, format.Node(&buf, restorer.Fset, astFile), "failed to format the restored file")
	return buf.String()
}

// RenderExpr renders a dst expression to a compact string. It supports the
// expression shapes used by call join point tests (identifiers, selectors,
// indexes, parentheses, literals, and pointers) and is intended to name callees
// in assertions, not to produce formatted Go source.
func RenderExpr(expr dst.Expr) string {
	switch expr := expr.(type) {
	case *dst.Ident:
		if expr.Path != "" {
			return expr.Path + "." + expr.Name
		}
		return expr.Name
	case *dst.SelectorExpr:
		return RenderExpr(expr.X) + "." + expr.Sel.Name
	case *dst.IndexExpr:
		return RenderExpr(expr.X) + "[" + RenderExpr(expr.Index) + "]"
	case *dst.IndexListExpr:
		parts := make([]string, len(expr.Indices))
		for i, idx := range expr.Indices {
			parts[i] = RenderExpr(idx)
		}
		return RenderExpr(expr.X) + "[" + strings.Join(parts, ", ") + "]"
	case *dst.ParenExpr:
		return "(" + RenderExpr(expr.X) + ")"
	case *dst.StarExpr:
		return "*" + RenderExpr(expr.X)
	case *dst.BasicLit:
		return expr.Value
	case *dst.CallExpr:
		args := make([]string, len(expr.Args))
		for i, arg := range expr.Args {
			args[i] = RenderExpr(arg)
		}
		return RenderExpr(expr.Fun) + "(" + strings.Join(args, ", ") + ")"
	case *dst.ArrayType:
		if expr.Len != nil {
			return "[" + RenderExpr(expr.Len) + "]" + RenderExpr(expr.Elt)
		}
		return "[]" + RenderExpr(expr.Elt)
	case *dst.MapType:
		return "map[" + RenderExpr(expr.Key) + "]" + RenderExpr(expr.Value)
	default:
		return fmt.Sprintf("<%T>", expr)
	}
}

// typeCheckResult carries the type-checked package and the file it was checked
// from, which the caller needs to decorate.
type typeCheckResult struct {
	*types.Package
	*types.Info
	AstFile *ast.File
}

func typeCheck(t *testing.T, fset *token.FileSet, imp types.Importer, importPath string, src string) typeCheckResult {
	t.Helper()

	result, err := tryTypeCheck(fset, imp, importPath, src)
	require.NoError(t, err, "failed to type-check %q", importPath)
	return result
}

// TypeCheckError type-checks src and returns the error for invalid source. It
// fails the test if the source is valid.
func TypeCheckError(t *testing.T, importPath string, src string) error {
	t.Helper()

	fset := token.NewFileSet()
	imp := &testImporter{fallback: importer.ForCompiler(fset, runtime.Compiler, nil), packages: nil}
	_, err := tryTypeCheck(fset, imp, importPath, src)
	require.Error(t, err, "expected %q to be rejected by the type checker", importPath)
	return err
}

// CheckSource type-checks a rendered source string, resolving the synthetic
// external packages registered by tests. It returns the type-check error, or
// nil for valid source.
func CheckSource(t *testing.T, importPath string, src string, extraSources map[string]string) error {
	t.Helper()

	fset := token.NewFileSet()
	imp := &testImporter{
		fallback: importer.ForCompiler(fset, runtime.Compiler, nil),
		packages: make(map[string]*types.Package, len(extraSources)),
	}
	for path, source := range extraSources {
		imp.packages[path] = typeCheck(t, fset, imp, path, source).Package
	}

	_, err := tryTypeCheck(fset, imp, importPath, src)
	return err
}

func tryTypeCheck(fset *token.FileSet, imp types.Importer, importPath string, src string) (typeCheckResult, error) {
	astFile, err := parser.ParseFile(fset, importPath+".go", src, parser.ParseComments)
	if err != nil {
		return typeCheckResult{}, err
	}

	info := &types.Info{
		Types:  make(map[ast.Expr]types.TypeAndValue),
		Uses:   make(map[*ast.Ident]types.Object),
		Scopes: make(map[ast.Node]*types.Scope),
	}
	pkg := types.NewPackage(importPath, astFile.Name.Name)
	// The repository module declares go 1.26. Pinning the language version here
	// matters for invalid-input tests: the Go 1.27 toolchain accepts experimental
	// generic methods at its own language version, and the injector passes the
	// compiled module's version to the type checker.
	checker := types.NewChecker(&types.Config{Importer: imp, GoVersion: "go1.26"}, fset, pkg, info)
	if err := checker.Files([]*ast.File{astFile}); err != nil {
		return typeCheckResult{}, err
	}

	return typeCheckResult{Package: pkg, Info: info, AstFile: astFile}, nil
}

// testImporter resolves the synthetic external packages registered by tests
// first, and falls back to the production compiler flavor for real imports,
// as the injector's own type checker does.
type testImporter struct {
	fallback types.Importer
	packages map[string]*types.Package
}

func (i *testImporter) Import(path string) (*types.Package, error) {
	if pkg, ok := i.packages[path]; ok {
		return pkg, nil
	}
	return i.fallback.Import(path)
}

var (
	_ types.Importer = (*testImporter)(nil)
)
