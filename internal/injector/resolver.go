// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package injector

import (
	"errors"
	"fmt"
	"go/importer"
	"go/token"
	"go/types"
	"runtime"

	"github.com/dave/dst/decorator/resolver"
	"golang.org/x/tools/go/gccgoexportdata"
	"golang.org/x/tools/go/gcexportdata"
)

// lookupResolver is a resolver.RestorerResolver that resolves package paths to package names using an importer.Lookup
// function.
type lookupResolver struct {
	lookup  importer.Lookup
	fset    *token.FileSet
	imports map[string]*types.Package
}

var _ resolver.RestorerResolver = (*lookupResolver)(nil)

// ResolvePackage retrieves the package name from the provided import path.
func (r *lookupResolver) ResolvePackage(path string) (string, error) {
	// Special case -- the "unsafe" package does not have an export file
	if path == "unsafe" {
		return "unsafe", nil
	}

	if pkg := r.imports[path]; pkg != nil {
		// We already resolved this package; so we'll just re-use the result.
		return pkg.Name(), nil
	}

	rd, err := r.lookup(path)
	if err != nil {
		return "", err
	}
	defer rd.Close()

	if r.fset == nil {
		r.fset = token.NewFileSet()
	}
	if r.imports == nil {
		r.imports = make(map[string]*types.Package)
	}

	switch runtime.Compiler {
	case "gc":
		rd, err := gcexportdata.NewReader(rd)
		if err != nil {
			return "", err
		}
		pkg, err := gcexportdata.Read(rd, r.fset, r.imports, path)
		if err != nil {
			return "", err
		}
		return pkg.Name(), nil
	case "gccgo":
		rd, err := gccgoexportdata.NewReader(rd)
		if err != nil {
			return "", err
		}
		pkg, err := gccgoexportdata.Read(rd, r.fset, r.imports, path)
		if err != nil {
			return "", err
		}
		return pkg.Name(), nil
	default:
		return "", fmt.Errorf("%s: %w", runtime.Compiler, errors.ErrUnsupported)
	}
}
