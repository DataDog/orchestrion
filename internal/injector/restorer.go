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

	"github.com/dave/dst/decorator"
	"golang.org/x/tools/go/gccgoexportdata"
	"golang.org/x/tools/go/gcexportdata"
)

type lookupResolver struct {
	lookup importer.Lookup

	fset    *token.FileSet
	imports map[string]*types.Package
}

func (i *Injector) newRestorer(filename string) *decorator.FileRestorer {
	if i.restorerResolver == nil {
		i.restorerResolver = &lookupResolver{lookup: i.Lookup}
	}

	return &decorator.FileRestorer{
		Restorer: decorator.NewRestorerWithImports(i.ImportPath, i.restorerResolver),
		Name:     filename,
	}
}

func (r *lookupResolver) ResolvePackage(path string) (string, error) {
	// The "unsafe" package does not have an archive, so it's hard-coded here.
	if path == "unsafe" {
		return "unsafe", nil
	}

	// If this is present in "cache", we can return right away!
	if pkg, ok := r.imports[path]; ok {
		return pkg.Name(), nil
	}

	if r.fset == nil {
		r.fset = token.NewFileSet()
	}
	if r.imports == nil {
		r.imports = make(map[string]*types.Package)
	}

	var err error
	for _, res := range resolvers {
		pkg, resolveErr := res(r.lookup, r.fset, r.imports, path)
		if resolveErr != nil {
			err = errors.Join(err, resolveErr)
			continue
		}
		return pkg.Name(), nil
	}

	return "", err
}

var resolvers []resolverFunc

type resolverFunc = func(importer.Lookup, *token.FileSet, map[string]*types.Package, string) (*types.Package, error)

func resolveGc(lookup importer.Lookup, fset *token.FileSet, imports map[string]*types.Package, path string) (*types.Package, error) {
	rd, err := lookup(path)
	if err != nil {
		return nil, err
	}
	defer rd.Close()

	gcr, err := gcexportdata.NewReader(rd)
	if err != nil {
		return nil, fmt.Errorf("locating gc export data: %w", err)
	}
	pkg, err := gcexportdata.Read(gcr, fset, imports, path)
	if err != nil {
		return nil, fmt.Errorf("reading gc export data: %w", err)
	}
	return pkg, nil
}

func resolveGccgo(lookup importer.Lookup, fset *token.FileSet, imports map[string]*types.Package, path string) (*types.Package, error) {
	rd, err := lookup(path)
	if err != nil {
		return nil, err
	}
	defer rd.Close()

	gcr, err := gccgoexportdata.NewReader(rd)
	if err != nil {
		return nil, fmt.Errorf("locating gc export data: %w", err)
	}
	pkg, err := gccgoexportdata.Read(gcr, fset, imports, path)
	if err != nil {
		return nil, fmt.Errorf("reading gc export data: %w", err)
	}
	return pkg, nil
}

func init() {
	// We assume code built with orchestrion is likely to use the same compiler as
	// the one used ot build orchestrion itself, so we'll attempt resolving using
	// that compiler's export data first.
	if runtime.Compiler == "gc" {
		resolvers = []resolverFunc{resolveGc, resolveGccgo}
	} else {
		resolvers = []resolverFunc{resolveGccgo, resolveGc}
	}
}
