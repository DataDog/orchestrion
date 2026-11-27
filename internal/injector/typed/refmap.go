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
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/DataDog/orchestrion/internal/injector/basiclit"
	"github.com/dave/dst"
)

type (
	// ReferenceKind denotes the style of a reference, which influences compilation and linking requirements.
	ReferenceKind bool

	// ReferenceMap associates import paths to ReferenceKind values.
	ReferenceMap struct {
		refs     map[string]ReferenceKind
		nodeMap  map[dst.Node]ast.Node
		scopes   map[ast.Node]*types.Scope
		requests map[string]*importRequest
	}
)

const (
	// ImportStatement references must be made available to the compiler via the provided `importcfg`.
	ImportStatement ReferenceKind = true
	// RelocationTarget references must be made available to the linker, and must be referenced (directly or not) by the main package.
	RelocationTarget ReferenceKind = false
)

// importRequest records a qualified import request made by advice for a path. Whether the file's
// own import can be reused, and which alias to generate when it cannot, is decided once the
// whole file has been woven: advice can introduce declarations that shadow names in ways the
// join-point scopes cannot see (e.g. a generated closure parameter, or a template-declared local).
type importRequest struct {
	// base is the alias base name from the first qualified request for the path.
	base string
	// blank is set when a blank import request was made for the path.
	blank bool
}

func NewReferenceMap(nodeMap map[dst.Node]ast.Node, scopes map[ast.Node]*types.Scope) ReferenceMap {
	return ReferenceMap{nodeMap: nodeMap, scopes: scopes}
}

// AddImport takes a package import path and the name in file and the result of a recursive parent lookup.
// It records the path as requiring an import statement. The decision whether the file's own import
// name for the path can be reused, and which alias to allocate when it cannot, is deferred to
// AddSyntheticImports, which runs after the whole file has been woven: that is the only point
// where every original and generated reference, and every declaration the advice introduced,
// is visible at once. It returns whether the request registered anything new.
func (r *ReferenceMap) AddImport(file *dst.File, nodes []dst.Node, path string, localName string) bool {
	if len(nodes) == 0 {
		panic("nodeChain must not be empty")
	}

	if r.requests == nil {
		r.requests = make(map[string]*importRequest)
	}
	req := r.requests[path]
	if req == nil {
		req = &importRequest{}
		r.requests[path] = req
	}

	if localName == "_" {
		// Blank request: the file already importing the path (under any name, dot, or blank)
		// means the package is imported and initialized already; nothing is left to do.
		if _, imported := hasImport(file, path); imported {
			return false
		}
		req.blank = true
		return r.add(path, ImportStatement)
	}

	if req.base == "" {
		// The first qualified spelling of the path wins, so repeated requests (one per generated
		// selector) and requests under different spellings stay deterministic.
		req.base = localName
	}
	return r.add(path, ImportStatement)
}

// AddLink registers the provided path as a relocation target resolution source. If this path is
// already registered as an import, this method does nothing and returns false.
func (r *ReferenceMap) AddLink(file *dst.File, path string) bool {
	if _, ok := hasImport(file, path); ok {
		return false
	}

	return r.add(path, RelocationTarget)
}

// AddSyntheticImports finalizes the import plan of the file: it decides, for every import path
// the advice requested, the single name the file will use for that path, and updates the
// file's import statements accordingly. It is not safe to call during an AST
// traversal by dstutil.Apply, as this may offset the declaration list by 1 in case a new import
// declaration needs to be added; call it after dstutil.Apply has returned.
//
// The restorer prints every reference to a path with the single name the file keeps for it, so
// that name must resolve to the intended package at every reference, original or generated. The
// file's own name is reused when it is safe everywhere; otherwise one generated alias is
// allocated for the whole file, with a deterministic numeric suffix whenever the base name is
// taken.
func (r *ReferenceMap) AddSyntheticImports(file *dst.File) bool {
	// Every path-bearing identifier of the final file, original and generated alike.
	refIdents := make(map[string][]*dst.Ident)
	dst.Inspect(file, func(node dst.Node) bool {
		if ident, ok := node.(*dst.Ident); ok && ident.Path != "" {
			refIdents[ident.Path] = append(refIdents[ident.Path], ident)
		}
		return true
	})

	// Plan the paths that advice requested. An import the advice never requests stays
	// untouched, even when a reference to it is shadowed: planning it would rewrite the
	// file's own import without the advice asking for it.
	if len(r.requests) == 0 {
		return false
	}

	scopes := newDstScopes(file)
	facts := r.fileFacts(file)

	var changed bool
	dropped := make(map[string]struct{})
	allocated := make(map[string]string)

	for _, path := range slices.Sorted(maps.Keys(r.requests)) {
		specs := importSpecsForPath(file, path)
		refs := refIdents[path]

		if len(specs) == 0 {
			// The file does not import the path yet.
			req := r.requests[path]
			if req.base == "" && req.blank {
				// Blank-only request: the path needs to be imported for its side effects.
				addSpec(file, &dst.ImportSpec{
					Path: importPathLit(path),
					Name: &dst.Ident{Name: "_"},
				})
			} else {
				sel := &aliasSelection{facts: facts, scopes: scopes, allocated: allocated, path: path, req: req, refs: refs}
				alias := sel.allocate("")
				addSpec(file, &dst.ImportSpec{
					Path: importPathLit(path),
					Name: &dst.Ident{Name: alias},
				})
				r.add(path, ImportStatement)
				allocated[path] = alias
			}
			changed = true
			continue
		}

		retained := RetainedImportSpec(specs)
		switch SpecName(retained) {
		case ".":
			// Dot import: the restorer prints the path's references bare. Keep the dot import
			// while every bare reference still resolves to the package; otherwise the whole
			// file must switch to a qualified name (the restorer supports one name per path).
			if anyRefShadowed(scopes, refs, func(ident *dst.Ident) string { return ident.Name }) {
				sel := &aliasSelection{facts: facts, scopes: scopes, allocated: allocated, path: path, req: r.requests[path], refs: refs}
				alias := sel.allocate("")
				retained.Name = &dst.Ident{Name: alias}
				r.add(path, ImportStatement)
				allocated[path] = alias
				changed = true
			}
			// The dot import itself is kept as-is, so the file stays untouched.
		case "_":
			// Blank import: it binds no name, so a qualified reference needs a named import.
			if len(refs) > 0 {
				sel := &aliasSelection{facts: facts, scopes: scopes, allocated: allocated, path: path, req: r.requests[path], refs: refs}
				alias := sel.allocate("")
				retained.Name = &dst.Ident{Name: alias}
				r.add(path, ImportStatement)
				allocated[path] = alias
				changed = true
			}
		default:
			actual := SpecName(retained)
			if actual == "" {
				// Unaliased: the real package name, from the type information of the original
				// package (never the path base or the template spelling).
				actual = facts.realNames[path]
			}
			if actual != "" && !anyRefShadowed(scopes, refs, func(*dst.Ident) string { return actual }) {
				// Pure reuse: the file's own name is safe at every reference. Nothing changes
				// for the file, and no synthetic dependency is required: the file already
				// imports the path under that name.
				dropped[path] = struct{}{}
				continue
			}
			sel := &aliasSelection{facts: facts, scopes: scopes, allocated: allocated, path: path, req: r.requests[path], refs: refs}
			alias := sel.allocate(actual)
			retained.Name = &dst.Ident{Name: alias}
			r.add(path, ImportStatement)
			allocated[path] = alias
			changed = true
		}
	}

	for path := range dropped {
		delete(r.refs, path)
	}
	if len(r.refs) == 0 {
		r.refs = nil
	}

	return changed
}

// aliasSelection carries the per-path inputs of alias allocation.
type aliasSelection struct {
	facts     *fileFacts
	scopes    *dstScopes
	allocated map[string]string
	path      string
	req       *importRequest
	refs      []*dst.Ident
}

// allocate returns the one generated alias for the selection's path. The candidate starts at
// __orchestrion_<name> (the request's spelling when there was one, otherwise the real package
// name) and appends a numeric suffix, starting at 1, until it is free: not a package-level
// name in any file of the package, not another import's name in this file, not already
// allocated, and not shadowed at any of the path's references.
func (sel *aliasSelection) allocate(retainedName string) string {
	name := ""
	switch {
	case sel.req.base != "" && sel.req.base != "_":
		name = sel.req.base
	case sel.facts.realNames[sel.path] != "":
		name = sel.facts.realNames[sel.path]
	case retainedName != "":
		name = retainedName
	default:
		name = sanitizeIdentifier(sel.path[strings.LastIndex(sel.path, "/")+1:])
	}

	base := "__orchestrion_" + name
	candidate := base
	for i := 1; sel.taken(candidate); i++ {
		candidate = base + strconv.Itoa(i)
	}
	return candidate
}

func (sel *aliasSelection) taken(name string) bool {
	if sel.facts.packageNames[name] {
		return true
	}
	// One alias per path per file: a candidate already allocated for any path is taken.
	for _, alias := range sel.allocated {
		if alias == name {
			return true
		}
	}
	if sel.facts.otherImportName(sel.path, name) {
		return true
	}
	return anyRefShadowed(sel.scopes, sel.refs, func(*dst.Ident) string { return name })
}

// anyRefShadowed reports whether the name produced by nameOf for a reference is declared in a
// scope enclosing that reference, so it would not resolve to a file-scope import there.
func anyRefShadowed(scopes *dstScopes, refs []*dst.Ident, nameOf func(*dst.Ident) string) bool {
	for _, ref := range refs {
		if scopes.shadowedAt(ref, nameOf(ref)) {
			return true
		}
	}
	return false
}

// fileFacts collects the facts about the package that alias selection needs and the original
// type information can provide: the package-level names declared anywhere in the package, the
// names each import path binds in this file, and the real package name behind each import path.
type fileFacts struct {
	packageNames map[string]bool
	// importNames maps each path imported by this file to the names it binds there.
	importNames map[string]map[string]bool
	// realNames maps import paths to their declared package name.
	realNames map[string]string
}

func (f *fileFacts) otherImportName(path string, name string) bool {
	for other, names := range f.importNames {
		if other == path {
			continue
		}
		if names[name] {
			return true
		}
	}
	return false
}

func (r *ReferenceMap) fileFacts(file *dst.File) *fileFacts {
	facts := &fileFacts{
		packageNames: make(map[string]bool),
		importNames:  make(map[string]map[string]bool),
		realNames:    make(map[string]string),
	}

	// Import names and paths from the file's own import statements.
	for _, spec := range file.Imports {
		path, err := basiclit.String(spec.Path)
		if err != nil {
			continue
		}
		names := facts.importNames[path]
		if names == nil {
			names = make(map[string]bool)
			facts.importNames[path] = names
		}
		if name := SpecName(spec); name != "" && name != "_" && name != "." {
			names[name] = true
		}
	}

	// The file scope of the type-checked original package: import names (including the real
	// names of unaliased imports) and the package scope, which holds the package-level names of
	// every file of the package.
	if r.scopes != nil && r.nodeMap != nil {
		if astFile, ok := r.nodeMap[file].(*ast.File); ok {
			if fileScope := r.scopes[astFile]; fileScope != nil {
				for _, name := range fileScope.Names() {
					if pkgName, ok := fileScope.Lookup(name).(*types.PkgName); ok {
						path := pkgName.Imported().Path()
						names := facts.importNames[path]
						if names == nil {
							names = make(map[string]bool)
							facts.importNames[path] = names
						}
						names[pkgName.Name()] = true
						facts.realNames[path] = pkgName.Imported().Name()
					}
				}
				// Package-level names from every file of the package. This covers declarations
				// that existed before injection; declarations injected into this very file are
				// covered by the same-file analysis in dstScopes.
				//
				// Known limitation (DataDog/orchestrion#950): declarations injected into
				// ANOTHER file of the same package are not visible here. Files are woven
				// concurrently, so by the time this file's alias is selected, a sibling file
				// may not have been woven yet. A package-level name injected into a sibling
				// file can therefore collide with the alias chosen for this file.
				if pkgScope := fileScope.Parent(); pkgScope != nil {
					for _, name := range pkgScope.Names() {
						facts.packageNames[name] = true
					}
				}
			}
		}
	}

	return facts
}

// addSpec appends an import spec to the file's last import declaration, creating one if the
// file has none, mirroring the historical behavior of AddSyntheticImports.
func addSpec(file *dst.File, spec *dst.ImportSpec) {
	var imports *dst.GenDecl
	for _, decl := range file.Decls {
		genDecl, ok := decl.(*dst.GenDecl)
		if !ok || genDecl.Tok != token.IMPORT {
			break
		}
		imports = genDecl
	}
	if imports == nil {
		imports = &dst.GenDecl{Tok: token.IMPORT}
		list := make([]dst.Decl, len(file.Decls)+1)
		list[0] = imports
		copy(list[1:], file.Decls)
		file.Decls = list
	}

	file.Imports = append(file.Imports, spec)
	imports.Specs = append(imports.Specs, spec)
}

func importPathLit(path string) *dst.BasicLit {
	return &dst.BasicLit{Kind: token.STRING, Value: fmt.Sprintf("%q", path)}
}

// hasImport checks if the provided file already imports the provided path and its local name.
func hasImport(file *dst.File, path string) (string, bool) {
	for _, spec := range file.Imports {
		specPath, err := basiclit.String(spec.Path)
		if err != nil {
			continue
		}
		if specPath == path {
			return SpecName(spec), true
		}
	}
	return "", false
}

func importSpecsForPath(file *dst.File, path string) []*dst.ImportSpec {
	var specs []*dst.ImportSpec
	for _, spec := range file.Imports {
		if specPath, err := basiclit.String(spec.Path); err == nil && specPath == path {
			specs = append(specs, spec)
		}
	}
	return specs
}

// sanitizeIdentifier turns an arbitrary path element into a component of a valid Go identifier.
func sanitizeIdentifier(element string) string {
	var b strings.Builder
	for _, r := range element {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_':
			_, _ = b.WriteRune(r)
		default:
			_ = b.WriteByte('_')
		}
	}
	return b.String()
}

func (r *ReferenceMap) Merge(other ReferenceMap) {
	for path, kind := range other.refs {
		r.add(path, kind)
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
