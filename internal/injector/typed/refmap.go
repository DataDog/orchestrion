// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package typed

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
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
		nodeMap map[dst.Node]ast.Node
		scopes  map[ast.Node]*types.Scope
	}
)

const (
	// ImportStatement references must be made available to the compiler via the provided `importcfg`.
	ImportStatement ReferenceKind = true
	// RelocationTarget references must be made available to the linker, and must be referenced (directly or not) by the main package.
	RelocationTarget ReferenceKind = false
)

func NewReferenceMap(nodeMap map[dst.Node]ast.Node, scopes map[ast.Node]*types.Scope) ReferenceMap {
	return ReferenceMap{nodeMap: nodeMap, scopes: scopes}
}

// AddImport takes a package import path and the name in file and the result of a recursive parent lookup.
// It first determines if the import is already present
// and if it has not been shadowed by a local declaration. If both conditions are met, the import is added to the
// reference map and the function returns true. Otherwise, it returns false.
func (r *ReferenceMap) AddImport(file *dst.File, nodes []dst.Node, path string, localName string) bool {
	if len(nodes) == 0 {
		panic("nodeChain must not be empty")
	}

	// If the import is already present, has a meaningful alias or no alias,
	// and is accessible from the current scope, we don't need to do anything.
	prevLocalName, ok := hasImport(file, path)
	if ok && prevLocalName != "." && prevLocalName != "_" && r.isImportInScope(nodes, path, localName) {
		return false
	}

	// Register in this ReferenceMap
	r.add(path, ImportStatement)
	if localName != "_" {
		// We don't register blank aliases, as this is the default behavior anyway...
		if r.aliases == nil {
			r.aliases = make(map[string]string)
		}
		r.aliases[path] = "__orchestrion_" + localName
	}

	return true
}

// isImportInScope checks if the provided name is an import in the scope of the provided node
func (r *ReferenceMap) isImportInScope(nodes []dst.Node, path string, name string) bool {
	if len(nodes) == 0 {
		panic("nodes must not be empty")
	}

	var (
		scope *types.Scope
		pos   = r.nodeMap[nodes[0]].Pos()
	)
	for i := 0; i < len(nodes) && scope == nil; i++ {
		node := nodes[i]
		if funcDecl, ok := node.(*dst.FuncDecl); ok {
			// Somehow scopes are not attached to FuncDecl nodes, so we need to look at the type ¯\_(シ)_/¯
			node = funcDecl.Type
		}

		astNode, ok := r.nodeMap[node]
		if !ok {
			continue
		}

		scope = r.scopes[astNode]
	}

	if scope == nil {
		panic(fmt.Errorf("unable to find scope for node %T in parent chain", nodes[0]))
	}

	_, obj := scope.LookupParent(name, pos)
	if obj != nil {
		if pkg, isImport := obj.(*types.PkgName); isImport {
			return pkg.Imported().Path() == path
		}
	}

	return false
}

// hasImport checks if the provided file already imports the provided path and its local name.
func hasImport(file *dst.File, path string) (string, bool) {
	for _, spec := range file.Imports {
		specPath, err := basiclit.String(spec.Path)
		if err != nil {
			continue
		}
		if specPath == path {
			name := ""
			if spec.Name != nil {
				name = spec.Name.Name
			}
			return name, true
		}
	}
	return "", false
}

// AddLink registers the provided path as a relocation target resolution source. If this path is
// already registered as an import, this method does nothing and returns false.
func (r *ReferenceMap) AddLink(file *dst.File, path string) bool {
	if _, ok := hasImport(file, path); ok {
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
