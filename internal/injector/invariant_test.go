// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package injector_test

import (
	"errors"
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/DataDog/orchestrion/internal/injector/aspect"
	"github.com/DataDog/orchestrion/internal/injector/parse"
	"github.com/dave/dst"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// This file implements the intended-binding invariant for golden fixtures. Type checking the
// emitted code is not enough: a local declaration can shadow an import alias and provide a method
// with the same name, so the emitted selector compiles but binds to the wrong object (see
// DataDog/orchestrion#950, C09). The invariant records, for every package reference of every
// modified file, the package it was intended to reference, then type checks the emitted output
// and verifies each reference still resolves there.
//
// Intended paths never come from the final dst.Ident.Path alone: for references that originate
// from the original source they come from an independent type check of the original sources (the
// binding oracle), keyed by file and position; for generated references they must be declared by
// the aspect configuration. A fix can therefore not certify itself.

type capturedRefKind uint8

const (
	// refInvalid is the zero value, so a capturedRef that carries only an error is never
	// mistaken for a reference of another kind.
	refInvalid capturedRefKind = iota
	refSelector
	refBareDot
	refSamePackage
)

type capturedRef struct {
	kind    capturedRefKind
	ordinal int
	path    string
	member  string
	invalid error
}

type oracleEntry struct {
	path      string
	member    string
	qualified bool // true: package-qualified selector; false: bare dot-imported identifier
}

// bindingOracle maps (filename, line, column) of every original package reference to its intended
// binding, as computed from an independent type check of the original sources.
type bindingOracle struct {
	entries       map[string]oracleEntry
	importPaths   map[string]bool // paths imported by the original files (any of them)
	originalFiles map[string]bool
}

func oracleKey(filename string, pos token.Position) string {
	return fmt.Sprintf("%s|%d|%d", filename, pos.Line, pos.Column)
}

// newBindingOracle type checks the original files and records the intended binding of every
// package-qualified selector and every dot-imported bare identifier.
func newBindingOracle(t *testing.T, files []string, lookup func(string) (io.ReadCloser, error), importPath string) *bindingOracle {
	t.Helper()

	fset := token.NewFileSet()
	astFiles := make([]*ast.File, 0, len(files))
	oracle := &bindingOracle{
		entries:       make(map[string]oracleEntry),
		importPaths:   make(map[string]bool),
		originalFiles: make(map[string]bool),
	}

	for _, file := range files {
		content, err := os.ReadFile(file)
		require.NoError(t, err)
		// Consume a leading //line directive exactly like the injector's parser does, so that
		// positions match regardless of the directive.
		mappedName := file
		if mapped, err := parse.ConsumeLineDirective(strings.NewReader(string(content))); err == nil && mapped != "" {
			mappedName = mapped
		}
		astFile, err := parser.ParseFile(fset, mappedName, content, parser.ParseComments)
		require.NoError(t, err, "failed to parse original file %q", file)
		astFiles = append(astFiles, astFile)
		oracle.originalFiles[mappedName] = true
		for _, imp := range astFile.Imports {
			path, err := strconv.Unquote(imp.Path.Value)
			require.NoError(t, err)
			oracle.importPaths[path] = true
		}
	}

	dotImports := make(map[string]bool)
	for _, astFile := range astFiles {
		for _, imp := range astFile.Imports {
			if imp.Name != nil && imp.Name.Name == "." {
				path, err := strconv.Unquote(imp.Path.Value)
				require.NoError(t, err)
				dotImports[path] = true
			}
		}
	}

	info := &types.Info{Uses: make(map[*ast.Ident]types.Object)}
	checker := types.Config{Importer: importer.ForCompiler(fset, "gc", lookup)}
	_, err := checker.Check(importPath, fset, astFiles, info)
	require.NoError(t, err, "original sources must type check")

	for _, astFile := range astFiles {
		filename := fset.Position(astFile.Pos()).Filename
		ast.Inspect(astFile, func(node ast.Node) bool {
			switch node := node.(type) {
			case *ast.SelectorExpr:
				ident, ok := node.X.(*ast.Ident)
				if !ok {
					return true
				}
				pkgName, ok := info.Uses[ident].(*types.PkgName)
				if !ok {
					return true
				}
				oracle.entries[oracleKey(filename, fset.Position(ident.Pos()))] = oracleEntry{
					path:      pkgName.Imported().Path(),
					member:    node.Sel.Name,
					qualified: true,
				}
			case *ast.Ident:
				obj := info.Uses[node]
				if obj == nil || obj.Pkg() == nil {
					return true
				}
				if !dotImports[obj.Pkg().Path()] {
					return true
				}
				oracle.entries[oracleKey(filename, fset.Position(node.Pos()))] = oracleEntry{
					path:      obj.Pkg().Path(),
					member:    obj.Name(),
					qualified: false,
				}
			}
			return true
		})
	}

	return oracle
}

// bindingEnv carries the inputs of the intended-binding invariant that are shared by all
// captured files of a fixture.
type bindingEnv struct {
	oracle          *bindingOracle
	declaredImports map[string]bool
	importPath      string
}

// captureArgs carries the per-file arguments of the restoration capture hook.
type captureArgs struct {
	filename   string
	file       *dst.File
	decorNodes map[dst.Node]ast.Node
	resNodes   map[dst.Node]ast.Node
	restored   *ast.File
	fset       *token.FileSet
}

// declaredImportPaths returns the set of import paths the aspects may introduce, per their
// declared imports. Generated references must reference one of these paths.
func declaredImportPaths(aspects []*aspect.Aspect) map[string]bool {
	declared := make(map[string]bool)
	// "unsafe" is implied by the aspect model itself (see aspect.Aspect.AddedImports, which
	// special-cases it), and linkname-based advice always depends on it.
	declared["unsafe"] = true
	for _, a := range aspects {
		for _, path := range a.AddedImports() {
			declared[path] = true
		}
	}
	return declared
}

// captureIntendedRefs records the intended binding of every package reference of a modified file,
// at restoration time. filename is the original source file name; file is the final dst tree;
// decoratorNodes maps nodes that originate from the original sources; restorerNodes maps every
// restored output node; restored is the restored ast tree; fset carries the original positions.
func captureIntendedRefs(env *bindingEnv, args captureArgs) ([]capturedRef, error) {
	file := args.file
	decoratorNodes := args.decorNodes
	restorerNodes := args.resNodes
	restored := args.restored
	fset := args.fset
	var errs []error

	// Ordinal maps for the restored output tree: every selector and every identifier gets a
	// preorder ordinal, so entries can be located again after the output is printed and re-parsed.
	selOrder := make(map[*ast.SelectorExpr]int)
	identOrder := make(map[*ast.Ident]int)
	selN, identN := 0, 0
	ast.Inspect(restored, func(node ast.Node) bool {
		switch node := node.(type) {
		case *ast.SelectorExpr:
			selOrder[node] = selN
			selN++
		case *ast.Ident:
			identOrder[node] = identN
			identN++
		}
		return true
	})

	refs := make([]capturedRef, 0, 16)
	dst.Inspect(file, func(node dst.Node) bool {
		ident, ok := node.(*dst.Ident)
		if !ok || ident.Path == "" {
			return true
		}

		restoredNode := restorerNodes[ident]
		if restoredNode == nil {
			errs = append(errs, fmt.Errorf("no restored node for package reference %s.%s (path %q)", ident.Name, "", ident.Path))
			return true
		}

		intendedPath, err := intendedPath(env, ident, decoratorNodes, fset)
		if err != nil {
			errs = append(errs, err)
			return true
		}

		switch restored := restoredNode.(type) {
		case *ast.SelectorExpr:
			refs = append(refs, capturedRef{kind: refSelector, ordinal: selOrder[restored], path: intendedPath, member: ident.Name})
		case *ast.Ident:
			if ident.Path == env.importPath {
				refs = append(refs, capturedRef{kind: refSamePackage, ordinal: identOrder[restored], path: intendedPath, member: ident.Name})
			} else {
				refs = append(refs, capturedRef{kind: refBareDot, ordinal: identOrder[restored], path: intendedPath, member: ident.Name})
			}
		default:
			errs = append(errs, fmt.Errorf("package reference %q restored to unexpected node %T", ident.Name, restoredNode))
		}
		return true
	})

	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	return refs, nil
}

// intendedPath determines the intended package path of a package reference, preferring the
// binding oracle (original sources, independent type check) over any state the fix can mutate.
func intendedPath(env *bindingEnv, ident *dst.Ident, decoratorNodes map[dst.Node]ast.Node, fset *token.FileSet) (string, error) {
	oracle := env.oracle
	declaredImports := env.declaredImports
	packageImportPath := env.importPath
	if orig, ok := decoratorNodes[ident]; ok {
		pos := fset.Position(orig.Pos())
		if oracle.originalFiles[pos.Filename] {
			key := oracleKey(pos.Filename, pos)
			if entry, found := oracle.entries[key]; found {
				if entry.path == ident.Path && entry.member == ident.Name {
					// The reference kept its original binding.
					return entry.path, nil
				}
				// The reference was re-purposed by advice (e.g. replace-function): the new
				// binding must be declared by the aspect configuration.
				if !declaredImports[ident.Path] {
					return "", fmt.Errorf("re-purposed reference %q at %s now points at %q, which is not declared by any aspect", ident.Name, key, ident.Path)
				}
				return ident.Path, nil
			}
			// An original position without an oracle entry can only be a reference created by
			// advice on an original node (e.g. replace-function); same constraint.
			if !declaredImports[ident.Path] {
				return "", fmt.Errorf("reference %q at %s points at %q, which is not declared by any aspect", ident.Name, key, ident.Path)
			}
			return ident.Path, nil
		}
	}
	// Generated or cloned reference: the path must be declared by the aspects (generated), or be
	// one of the original imports (clones of original expressions embedded in templates).
	if !declaredImports[ident.Path] && !oracle.importPaths[ident.Path] && ident.Path != packageImportPath {
		return "", fmt.Errorf("generated reference %q points at %q, which is not declared by any aspect nor imported originally", ident.Name, ident.Path)
	}
	return ident.Path, nil
}

// checkPackageBindings type checks the emitted package and verifies every captured
// reference still resolves to its intended package. It fails the test if the package does
// not type check or if any reference binds elsewhere.
func checkPackageBindings(t *testing.T, files []string, captured map[string][]capturedRef, lookup func(string) (io.ReadCloser, error), importPath string) {
	t.Helper()

	if len(captured) == 0 {
		return
	}

	errs := verifyBindings(files, captured, lookup, importPath)
	require.Empty(t, errs, "binding invariant violated")
	t.Logf("binding invariant: %d package references verified across %d files", countCaptured(captured), len(captured))
}

func countCaptured(captured map[string][]capturedRef) int {
	total := 0
	for _, refs := range captured {
		total += len(refs)
	}
	return total
}

// verifyBindings type checks the emitted package files together and verifies every captured
// reference resolves to its intended package. It returns the list of violations; an empty list
// means the emitted output is sound.
func verifyBindings(files []string, captured map[string][]capturedRef, lookup func(string) (io.ReadCloser, error), importPath string) []error {
	var errs []error
	fail := func(format string, args ...any) {
		errs = append(errs, fmt.Errorf(format, args...))
	}

	fset := token.NewFileSet()
	astFiles := make([]*ast.File, 0, len(files))
	for _, file := range files {
		astFile, err := parser.ParseFile(fset, file, nil, parser.ParseComments)
		if err != nil {
			errs = append(errs, fmt.Errorf("emitted output must parse %q: %w", file, err))
			return errs
		}
		astFiles = append(astFiles, astFile)
	}

	info := &types.Info{Uses: make(map[*ast.Ident]types.Object)}
	checker := types.Config{Importer: importer.ForCompiler(fset, "gc", lookup)}
	if _, err := checker.Check(importPath, fset, astFiles, info); err != nil {
		errs = append(errs, fmt.Errorf("emitted output must type check: %w", err))
		return errs
	}

	checked := 0
	for filename, refs := range captured {
		outAsts := make([]*ast.File, 0, len(astFiles))
		// A leading //line directive in the emitted output can remap the parsed positions to a
		// relative file name, so compare base names; fixture files have unique base names.
		for _, astFile := range astFiles {
			if filepath.Base(fset.Position(astFile.Pos()).Filename) == filepath.Base(filename) {
				outAsts = append(outAsts, astFile)
			}
		}
		if len(outAsts) != 1 {
			fail("expected exactly one emitted file named %q, found %d", filename, len(outAsts))
			continue
		}

		selOrder := make(map[*ast.SelectorExpr]int)
		identOrder := make(map[*ast.Ident]int)
		selN, identN := 0, 0
		ast.Inspect(outAsts[0], func(node ast.Node) bool {
			switch node := node.(type) {
			case *ast.SelectorExpr:
				selOrder[node] = selN
				selN++
			case *ast.Ident:
				identOrder[node] = identN
				identN++
			}
			return true
		})

		for _, ref := range refs {
			switch ref.kind {
			case refSelector:
				outSel := lookupSelectorByOrdinal(selOrder, ref.ordinal)
				if outSel == nil {
					fail("%s: no selector with ordinal %d in emitted output", filename, ref.ordinal)
					continue
				}
				qualifier, ok := outSel.X.(*ast.Ident)
				if !ok {
					fail("%s: package reference must have an identifier qualifier", filename)
					continue
				}
				pkgName, ok := info.Uses[qualifier].(*types.PkgName)
				if !ok {
					fail("%s: selector %s.%s (ordinal %d) must resolve to package %s, got %T", filename, qualifier.Name, outSel.Sel.Name, ref.ordinal, ref.path, info.Uses[qualifier])
					continue
				}
				if pkgName.Imported().Path() != ref.path {
					fail("%s: selector %s.%s must resolve to package %s", filename, qualifier.Name, outSel.Sel.Name, ref.path)
					continue
				}
				if outSel.Sel.Name != ref.member {
					fail("%s: selector member must remain %q", filename, ref.member)
					continue
				}
				checked++
			case refBareDot, refSamePackage:
				outIdent := lookupIdentByOrdinal(identOrder, ref.ordinal)
				if outIdent == nil {
					fail("%s: no identifier with ordinal %d in emitted output", filename, ref.ordinal)
					continue
				}
				obj := info.Uses[outIdent]
				if obj == nil || obj.Pkg() == nil {
					fail("%s: bare reference %q must resolve to a package object", filename, outIdent.Name)
					continue
				}
				if obj.Pkg().Path() != ref.path {
					fail("%s: bare reference %q must resolve to package %s", filename, outIdent.Name, ref.path)
					continue
				}
				checked++
			case refInvalid:
				fail("%s: capture failed: %v", filename, ref.invalid)
			}
		}
	}

	// Structural guard: generated code must keep resolving predeclared identifiers (e.g.
	// append) to the universe scope, which rejects alias schemes that would shadow them.
	for _, astFile := range astFiles {
		ast.Inspect(astFile, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			ident, ok := call.Fun.(*ast.Ident)
			if !ok || ident.Name != "append" {
				return true
			}
			if _, isBuiltin := info.Uses[ident].(*types.Builtin); !isBuiltin {
				fail("append must resolve to the universe scope, got %T", info.Uses[ident])
			}
			return true
		})
	}

	if total := countCaptured(captured); checked != total {
		fail("every captured package reference must be verified: checked %d of %d", checked, total)
	}

	return errs
}

func lookupSelectorByOrdinal(order map[*ast.SelectorExpr]int, ordinal int) *ast.SelectorExpr {
	for sel, ord := range order {
		if ord == ordinal {
			return sel
		}
	}
	return nil
}

func lookupIdentByOrdinal(order map[*ast.Ident]int, ordinal int) *ast.Ident {
	for ident, ord := range order {
		if ord == ordinal {
			return ident
		}
	}
	return nil
}

// TestBindingInvariantNegativeControl (C09 / R(SilentWrongBinding)) proves the binding
// invariant itself catches a silently wrong binding: a captured entry whose intended path
// does not match the emitted binding must produce a violation, even though the file
// compiles and the selector resolves to a local method.
func TestBindingInvariantNegativeControl(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "input.go")
	source := `package test

type local struct{}

func (local) Floor() int { return 1 }

var v = local{}
var _ = v.Floor()
`
	require.NoError(t, os.WriteFile(file, []byte(source), 0o644))

	lookup := func(path string) (io.ReadCloser, error) {
		return nil, fmt.Errorf("no import expected in negative control, got %q", path)
	}

	// v.Floor is selector 0 of the file; it binds to the local method, not to a package.
	captured := map[string][]capturedRef{
		file: {{kind: refSelector, ordinal: 0, path: "strings", member: "Floor"}},
	}

	errs := verifyBindings([]string{file}, captured, lookup, "dummy/test/module")
	require.NotEmpty(t, errs, "the invariant must catch the wrong binding")
	assert.Contains(t, errs[0].Error(), "must resolve to package strings")
}
