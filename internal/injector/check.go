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

	"github.com/DataDog/dd-trace-go/v2/ddtrace/tracer"
	"github.com/DataDog/orchestrion/internal/injector/parse"
)

// typeCheck runs the Go type checker on the provided files, and returns the
// Uses type information map that is built in the process.
func (i *Injector) typeCheck(ctx context.Context, fset *token.FileSet, files []parse.File) (_ types.Info, err error) {
	span, _ := tracer.StartSpanFromContext(ctx, "Injector.typeCheck")
	defer func() { span.Finish(tracer.WithError(err)) }()

	pkg := types.NewPackage(i.ImportPath, i.Name)
	typeInfo := types.Info{
		Types:  make(map[ast.Expr]types.TypeAndValue),
		Uses:   make(map[*ast.Ident]types.Object),
		Scopes: make(map[ast.Node]*types.Scope),
	}

	checkerCfg := types.Config{
		GoVersion: i.GoVersion,
		Importer:  importer.ForCompiler(fset, runtime.Compiler, i.Lookup),
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
			return types.Info{}, fmt.Errorf("orchestrion was built with Go version %s but package %q requires a newer go version, please reinstall and pin orchestrion with a newer Go version: type-checking files: %w", runtime.Version(), i.ImportPath, err)
		}

		return types.Info{}, typeCheckingError{cause: err}
	}

	return typeInfo, nil
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
