// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package injector

import (
	"go/importer"
	"go/token"
	"go/types"

	"github.com/dave/dst/decorator"
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

	pkg, err := r.readPackageInfo(rd, path)
	if err != nil {
		return "", err
	}
	return pkg.Name(), nil
}
