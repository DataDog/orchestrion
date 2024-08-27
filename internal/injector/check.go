// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package injector

import (
	"fmt"
	"go/ast"
	"go/importer"
	"go/token"
	"go/types"
	"runtime"
)

// typeCheck runs the Go type checker on the provided files, and returns the
// Uses type information map that is built in the process.
func (i *Injector) typeCheck(fset *token.FileSet, files []*ast.File) (map[*ast.Ident]types.Object, error) {
	pkg := types.NewPackage(i.ImportPath, i.Name)
	typeInfo := types.Info{Uses: make(map[*ast.Ident]types.Object)}

	checkerCfg := types.Config{
		GoVersion: i.GoVersion,
		Importer:  importer.ForCompiler(fset, runtime.Compiler, i.Lookup),
	}
	checker := types.NewChecker(&checkerCfg, fset, pkg, &typeInfo)

	if err := checker.Files(files); err != nil {
		return nil, fmt.Errorf("type-checking files: %w", err)
	}

	return typeInfo.Uses, nil
}
