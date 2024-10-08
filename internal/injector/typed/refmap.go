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

	"github.com/DataDog/orchestrion/internal/injector/basiclit"
	"github.com/dave/dst"
)

type (
	// ReferenceKind denotes the style of a reference, which influences compilation and linking requirements.
	ReferenceKind bool

	// ReferenceMap associates import paths to ReferenceKind values.
	ReferenceMap struct {
		refs    map[string]ReferenceKind
		aliases map[string]string
	}
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
func (r *ReferenceMap) AddImport(file *dst.File, path string, alias string) bool {
	if hasImport(file, path) {
		return false
	}

	// Register in this ReferenceMap
	r.add(path, ImportStatement)
	if alias != "_" {
		// We don't register blank aliases, as this is the default behavior anyway...
		if r.aliases == nil {
			r.aliases = make(map[string]string)
		}
		r.aliases[path] = fmt.Sprintf("__orchestrion_%s", alias)
	}

	return true
}

func hasImport(file *dst.File, path string) bool {
	for _, spec := range file.Imports {
		specPath, err := basiclit.String(spec.Path)
		if err != nil {
			continue
		}
		if specPath == path {
			return true
		}
	}
	return false
}

// AddLink registers the provided path as a relocation target resolution source. If this path is
// already registered as an import, this method does nothing and returns false.
func (r *ReferenceMap) AddLink(file *dst.File, path string) bool {
	if hasImport(file, path) {
		return false
	}

	return r.add(path, RelocationTarget)
}

// AddSyntheticImports adds the registered imports to the provided *dst.File. This is not safe to
// call during an AST traversal by dstutil.Apply, as this may offset the declaration list by 1 in
// case a new import declaration needs to be added, which would result in re-traversing current
// declaration when the cursor moves forward. Instead, it is advise to call this method after
// dstutil.Apply has returned.
func (r *ReferenceMap) AddSyntheticImports(file *dst.File) bool {
	toAdd := make([]*dst.ImportSpec, 0, len(r.refs))

	for path, kind := range r.refs {
		if kind != ImportStatement {
			continue
		}
		name := &dst.Ident{Name: "_"}
		if alias := r.aliases[path]; alias != "" {
			name.Name = alias
		}
		toAdd = append(toAdd, &dst.ImportSpec{Path: &dst.BasicLit{Kind: token.STRING, Value: fmt.Sprintf("%q", path)}, Name: name})
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
		genDecl, ok := decl.(*dst.GenDecl)
		if !ok || genDecl.Tok != token.IMPORT {
			break
		}
		imports = genDecl
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
	for path, kind := range other.refs {
		r.add(path, kind)
		if alias := other.aliases[path]; alias != "" {
			if r.aliases == nil {
				r.aliases = make(map[string]string)
			}
			r.aliases[path] = alias
		}
	}
}

func (r *ReferenceMap) Map() map[string]ReferenceKind {
	return r.refs
}

func (r *ReferenceMap) Count() int {
	return len(r.refs)
}

func (r *ReferenceMap) add(path string, kind ReferenceKind) bool {
	if r.refs == nil {
		r.refs = map[string]ReferenceKind{path: kind}
		return true
	} else if old, found := r.refs[path]; !found || old != ImportStatement {
		// If it was already in as an ImportStatement, we don't do anything, since that is the strongest
		// kind of reference (imported implies relocatable, the reverse is not true).
		r.refs[path] = kind
		return true
	}
	return false
}

func (k ReferenceKind) String() string {
	if k == ImportStatement {
		return "ImportStatement"
	}
	return "RelocationTarget"
}
