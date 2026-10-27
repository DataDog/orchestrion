// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package injector

import (
	"context"
	"fmt"
	"go/ast"
	"go/importer"
	"go/token"
	"go/types"
	"runtime"
	"strings"
	"sync"

	"github.com/DataDog/dd-trace-go/v2/ddtrace/tracer"
	"github.com/DataDog/orchestrion/internal/injector/parse"
)

// typeCheckResult carries the output of [Injector.typeCheck]: the type information for the
// package's files, and the importer that was used to resolve package types. The importer is safe
// for concurrent use, and should be used by advice & join points to resolve types consistently
// with what the type checker (and, transitively, the compiler) sees.
type typeCheckResult struct {
	types.Info
	Importer types.Importer
}

// typeCheck runs the Go type checker on the provided files, and returns the
// Uses type information map that is built in the process, along with the
// importer that was used to resolve package types.
func (i *Injector) typeCheck(ctx context.Context, fset *token.FileSet, files []parse.File) (_ typeCheckResult, err error) {
	span, _ := tracer.StartSpanFromContext(ctx, "Injector.typeCheck")
	defer func() { span.Finish(tracer.WithError(err)) }()

	pkg := types.NewPackage(i.ImportPath, i.Name)
	typeInfo := types.Info{
		Types:  make(map[ast.Expr]types.TypeAndValue),
		Uses:   make(map[*ast.Ident]types.Object),
		Scopes: make(map[ast.Node]*types.Scope),
	}

	imp := &lockedImporter{imp: importer.ForCompiler(fset, runtime.Compiler, i.Lookup)}
	checkerCfg := types.Config{
		GoVersion: i.GoVersion,
		Importer:  imp,
	}
	checker := types.NewChecker(&checkerCfg, fset, pkg, &typeInfo)

	astFiles := make([]*ast.File, len(files))
	for i, file := range files {
		astFiles[i] = file.AstFile
	}

	if err := checker.Files(astFiles); err != nil {
		// This is a workaround for the fact that the Go type checker does not return a specific unexported error type
		// TODO: Ask better error typing from the Go team for the go/types package
		if strings.Contains(err.Error(), "package requires newer Go version") {
			// Not returning a type-checking error here, as this error we want to surface directly to the user ourselves.
			return typeCheckResult{}, fmt.Errorf("orchestrion was built with Go version %s but package %q requires a newer go version, please reinstall and pin orchestrion with a newer Go version: type-checking files: %w", runtime.Version(), i.ImportPath, err)
		}

		return typeCheckResult{}, typeCheckingError{cause: err}
	}

	return typeCheckResult{Info: typeInfo, Importer: imp}, nil
}

// lockedImporter serializes access to a [types.Importer], as the implementations provided by
// [go/importer] are not safe for concurrent use (they mutate a shared package cache), while the
// weaving process evaluates files concurrently.
type lockedImporter struct {
	mu  sync.Mutex
	imp types.Importer
}

var (
	_ types.Importer = (*lockedImporter)(nil)
)

func (i *lockedImporter) Import(path string) (*types.Package, error) {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.imp.Import(path)
}

type typeCheckingError struct {
	cause error
}

var _ error = typeCheckingError{}

func (e typeCheckingError) Error() string {
	return fmt.Sprintf("type-checking files: %v", e.cause)
}

func (typeCheckingError) Is(target error) bool {
	switch target.(type) {
	case typeCheckingError:
		return true
	default:
		return false
	}
}

func (e typeCheckingError) Unwrap() error {
	return e.cause
}
