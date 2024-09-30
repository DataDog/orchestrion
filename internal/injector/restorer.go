// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package injector

import (
	"fmt"
	"go/importer"
	"go/token"
	"go/types"
	"sync"

	"github.com/dave/dst/decorator"
	"golang.org/x/tools/go/gcexportdata"
)

type lookupResolver struct {
	lookup importer.Lookup

	fset    *token.FileSet
	imports map[string]*types.Package

	mu sync.Mutex
}

func (i *Injector) newRestorer(filename string) *decorator.FileRestorer {
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

	r.mu.Lock()
	defer r.mu.Unlock()

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

	rd, err := r.lookup(path)
	if err != nil {
		return "", fmt.Errorf("lookup %q: %w", path, err)
	}
	gcr, err := gcexportdata.NewReader(rd)
	if err != nil {
		return "", fmt.Errorf("locating gc export data: %w", err)
	}
	pkg, err := gcexportdata.Read(gcr, r.fset, r.imports, path)
	if err != nil {
		return "", fmt.Errorf("reading gc export data: %w", err)
	}

	return pkg.Name(), err
}
