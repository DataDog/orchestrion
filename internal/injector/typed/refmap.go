// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package typed

import (
	"fmt"
	"go/token"

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
// available within the specified file. If so, it adds the import to the file, registers it in this
// ReferenceMap, and returns true. Otherwise it returns falses.
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

	// Add the missing import to the file
	spec := &dst.ImportSpec{Path: &dst.BasicLit{Kind: token.STRING, Value: fmt.Sprintf("%q", path)}}
	file.Imports = append(file.Imports, spec)
	var imports *dst.GenDecl
	for _, decl := range file.Decls {
		if genDecl, ok := decl.(*dst.GenDecl); ok && genDecl.Tok == token.IMPORT {
			imports = genDecl
		} else {
			break
		}
	}
	if imports == nil {
		imports = &dst.GenDecl{Tok: token.IMPORT}
		list := make([]dst.Decl, 1, len(file.Decls)+1)
		list[0] = imports
		file.Decls = append(list, file.Decls...)
	}
	imports.Specs = append(imports.Specs, spec)

	// Register in this ReferenceMap
	r.add(path, ImportStatement)

	return true
}

func (r *ReferenceMap) add(path string, kind ReferenceKind) {
	if *r == nil {
		*r = ReferenceMap{path: kind}
	} else {
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
