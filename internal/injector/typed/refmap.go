// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package typed

import (
	"fmt"
	"go/token"
	"slices"
	"strings"

	"github.com/datadog/orchestrion/internal/injector/basiclit"
	"github.com/dave/dst"
)

type (
	// ReferenceKind denotes the style of a reference, which influences compilation and linking requirements.
	ReferenceKind bool

	// ReferenceMap associates import paths to ReferenceKind values.
	ReferenceMap map[string]ReferenceKind
)

const (
	// ImportStatement references must be made available to the compiler via the provided `importcfg`.
	ImportStatement ReferenceKind = true
	// RelocationTarget references must be made available to the linker, and must be referenced (directly or not) by the main package.
	RelocationTarget ReferenceKind = false
)

// AddImport determines whether a new import declaration needs to be added to make the provided path
// available within the specified file. Returns true if that is the case. False if the import path
// is already available within the file.
func (r *ReferenceMap) AddImport(file *dst.File, path string) bool {
	// Browse the current file to see if the import already exists...
	for _, spec := range file.Imports {
		specPath, err := basiclit.String(spec.Path)
		if err != nil {
			continue
		}
		if specPath == path {
			return false
		}
	}

	// Register in this ReferenceMap
	r.add(path, ImportStatement)

	return true
}

// AddSyntheticImports adds the registered imports to the provided *dst.File. This is not safe to
// call during an AST traversal by dstutil.Apply, as this may offset the declaration list by 1 in
// case a new import declaration needs to be added, which would result in re-traversing current
// declaration when the cursor moves forward. Instead, it is advise to call this method after
// dstutil.Apply has returned.
func (r *ReferenceMap) AddSyntheticImports(file *dst.File) bool {
	toAdd := make([]*dst.ImportSpec, 0, len(*r))

	for path, kind := range *r {
		if kind != ImportStatement {
			continue
		}
		toAdd = append(toAdd, &dst.ImportSpec{Path: &dst.BasicLit{Kind: token.STRING, Value: fmt.Sprintf("%q", path)}})
	}

	if len(toAdd) == 0 {
		return false
	}

	// Sort the import specs to ensure deterministic output.
	slices.SortFunc(toAdd, func(l, r *dst.ImportSpec) int {
		return strings.Compare(l.Path.Value, r.Path.Value)
	})

	// Find the last import declaration in the file...
	var imports *dst.GenDecl
	for _, decl := range file.Decls {
		if genDecl, ok := decl.(*dst.GenDecl); ok && genDecl.Tok == token.IMPORT {
			imports = genDecl
		} else {
			break
		}
	}
	// ...or create a new one if none is found...
	if imports == nil {
		imports = &dst.GenDecl{Tok: token.IMPORT}
		list := make([]dst.Decl, len(file.Decls)+1)
		list[0] = imports
		copy(list[1:], file.Decls)
		file.Decls = list
	}

	// Add the necessary imports
	file.Imports = append(file.Imports, toAdd...)
	newSpecs := make([]dst.Spec, len(toAdd))
	for i, spec := range toAdd {
		newSpecs[i] = spec
	}
	imports.Specs = append(imports.Specs, newSpecs...)

	return true
}

func (r *ReferenceMap) Merge(other ReferenceMap) {
	for path, kind := range other {
		r.add(path, kind)
	}
}

func (r *ReferenceMap) add(path string, kind ReferenceKind) {
	if *r == nil {
		*r = ReferenceMap{path: kind}
	} else {
		if prev, found := (*r)[path]; found {
			if prev == ImportStatement {
				return
			}
		}
		(*r)[path] = kind
	}
}

func (k ReferenceKind) String() string {
	if k == ImportStatement {
		return "ImportStatement"
	} else {
		return "RelocationTarget"
	}
}
