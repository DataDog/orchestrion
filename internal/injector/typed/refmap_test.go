// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package typed

import (
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"maps"
	"slices"
	"testing"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
	"github.com/dave/dst/decorator/resolver/gotypes"
	"github.com/dave/dst/dstutil"
	"github.com/stretchr/testify/require"
)

// This file tests the import-alias planning of ReferenceMap (DataDog/orchestrion#950). All
// scenarios are driven through the public API (AddImport followed by AddSyntheticImports) so the
// assertions capture behavior, not implementation details:
//
//   - "reuse" means the file's own import name is kept as-is, the spec set for the path is
//     unchanged, and no synthetic reference is registered;
//   - "generate" means the retained spec for the path carries the generated alias (renamed in
//     place, no duplicate spec), and a synthetic reference of kind ImportStatement is registered.
//
// Generated references are simulated by wrapping the os.Getenv join-point call, exactly like
// wrap-expression advice does. The scopes of the original source come from a real type check, so
// the shadowing shapes reflect go/types semantics. The scenario IDs map to the rows of the #950
// test matrix (see /tmp/orch-bash/950/test-matrix.md and the adversarial review).

var sharedImporter = importer.Default()

type srcFile struct {
	name string
	code string
}

func src(code string) srcFile { return srcFile{name: "input.go", code: code} }

// extra is a support file of the same package.
func extra(code string) srcFile { return srcFile{name: "extra.go", code: code} }

// fileEnv carries the dst tree of one fixture file along with the maps needed to drive
// ReferenceMap the way the injector does.
type fileEnv struct {
	file    *dst.File
	dec     *decorator.Decorator
	parents map[dst.Node]dst.Node
	scopes  map[ast.Node]*types.Scope
	nodeMap map[dst.Node]ast.Node
}

// decoratePackage type checks the fixture files together (as one package) and decorates each of
// them with its own decorator, mirroring the injector's per-file setup.
func decoratePackage(t testing.TB, files []srcFile) []fileEnv {
	t.Helper()

	fset := token.NewFileSet()
	astFiles := make([]*ast.File, len(files))
	for i, file := range files {
		astFile, err := parser.ParseFile(fset, file.name, file.code, parser.ParseComments)
		require.NoError(t, err, "failed to parse %q", file.name)
		astFiles[i] = astFile
	}

	info := types.Info{
		Uses:   make(map[*ast.Ident]types.Object),
		Scopes: make(map[ast.Node]*types.Scope),
	}
	checker := types.Config{Importer: sharedImporter}
	_, err := checker.Check("dummy/test/module", fset, astFiles, &info)
	require.NoError(t, err, "fixture sources must type check")

	envs := make([]fileEnv, len(files))
	for i, astFile := range astFiles {
		dec := decorator.NewDecoratorWithImports(fset, "dummy/test/module", gotypes.New(info.Uses))
		file, err := dec.DecorateFile(astFile)
		require.NoError(t, err, "failed to decorate %q", files[i].name)

		parents := make(map[dst.Node]dst.Node)
		dstutil.Apply(file, func(csor *dstutil.Cursor) bool {
			if csor.Node() != nil && csor.Parent() != nil {
				parents[csor.Node()] = csor.Parent()
			}
			return true
		}, nil)

		envs[i] = fileEnv{
			file:    file,
			dec:     dec,
			parents: parents,
			scopes:  info.Scopes,
			nodeMap: dec.Ast.Nodes,
		}
	}
	return envs
}

// joinCall finds the n-th os.Getenv(...) call in the file.
func (e *fileEnv) joinCall(t testing.TB, n int) *dst.CallExpr {
	t.Helper()

	var found *dst.CallExpr
	count := 0
	dst.Inspect(e.file, func(node dst.Node) bool {
		if found != nil {
			return false
		}
		call, ok := node.(*dst.CallExpr)
		if !ok {
			return true
		}
		if fun, ok := call.Fun.(*dst.Ident); ok && fun.Path == "os" && fun.Name == "Getenv" {
			if count == n {
				found = call
				return false
			}
			count++
		}
		return true
	})
	require.NotNil(t, found, "join call #%d not found", n)
	return found
}

// nodeChain returns the node chain from node up to the file, as the advice context builds it.
func (e *fileEnv) nodeChain(node dst.Node) []dst.Node {
	chain := []dst.Node{node}
	for {
		parent, ok := e.parents[node]
		if !ok || parent == e.file {
			break
		}
		chain = append(chain, parent)
		node = parent
	}
	return append(chain, e.file)
}

// simulateWrap replaces the n-th join-point call with a generated reference to path.member, the
// way wrap-expression advice does. If closureLocal is not empty, the generated reference sits in
// a generated closure that first declares a local with that name (C11/F04 shapes). The chain of
// the original call is returned for AddImport.
func (e *fileEnv) simulateWrap(t *testing.T, n int, path string, member string, closureLocal string) []dst.Node {
	t.Helper()

	call := e.joinCall(t, n)
	chain := e.nodeChain(call)

	var wrapper dst.Expr = &dst.CallExpr{
		Fun:  &dst.Ident{Path: path, Name: member},
		Args: []dst.Expr{call},
	}
	if closureLocal != "" {
		wrapper = &dst.CallExpr{
			Fun: &dst.FuncLit{
				Type: &dst.FuncType{Results: &dst.FieldList{List: []*dst.Field{{Type: dst.NewIdent("string")}}}},
				Body: &dst.BlockStmt{List: []dst.Stmt{
					&dst.AssignStmt{
						Lhs: []dst.Expr{dst.NewIdent(closureLocal)},
						Tok: token.DEFINE,
						Rhs: []dst.Expr{&dst.BasicLit{Kind: token.STRING, Value: `""`}},
					},
					&dst.AssignStmt{Lhs: []dst.Expr{dst.NewIdent("_")}, Tok: token.ASSIGN, Rhs: []dst.Expr{dst.NewIdent(closureLocal)}},
					&dst.ReturnStmt{Results: []dst.Expr{wrapper}},
				}},
			},
		}
	}

	replaced := false
	dstutil.Apply(e.file, func(csor *dstutil.Cursor) bool {
		if csor.Node() == call {
			csor.Replace(wrapper)
			replaced = true
			return false
		}
		return !replaced
	}, nil)
	require.True(t, replaced, "failed to replace join call #%d", n)

	return chain
}

// simulatePrepend inserts a generated `name := "generated"; _ = name` at the top of the body of
// the function with the given name, the way prepend-statements advice does.
func (e *fileEnv) simulatePrepend(t *testing.T, funcName string, name string) {
	t.Helper()

	var body *dst.BlockStmt
	dst.Inspect(e.file, func(node dst.Node) bool {
		if decl, ok := node.(*dst.FuncDecl); ok && decl.Name.Name == funcName {
			body = decl.Body
			return false
		}
		return body == nil
	})
	require.NotNil(t, body, "function %q not found", funcName)

	generated := []dst.Stmt{
		&dst.AssignStmt{
			Lhs: []dst.Expr{dst.NewIdent(name)},
			Tok: token.DEFINE,
			Rhs: []dst.Expr{&dst.BasicLit{Kind: token.STRING, Value: `"generated"`}},
		},
		&dst.AssignStmt{Lhs: []dst.Expr{dst.NewIdent("_")}, Tok: token.ASSIGN, Rhs: []dst.Expr{dst.NewIdent(name)}},
	}
	body.List = append(generated, body.List...)
}

// simulateAddField appends a generated field whose type references path to the struct type
// with the given name, the way add-struct-field advice does.
func (e *fileEnv) simulateAddField(t *testing.T, typeName string, path string) {
	t.Helper()

	var fields *dst.FieldList
	dst.Inspect(e.file, func(node dst.Node) bool {
		if spec, ok := node.(*dst.TypeSpec); ok && spec.Name.Name == typeName {
			if structType, ok := spec.Type.(*dst.StructType); ok {
				fields = structType.Fields
				return false
			}
		}
		return fields == nil
	})
	require.NotNil(t, fields, "struct type %q not found", typeName)

	fields.List = append(fields.List, &dst.Field{
		Names: []*dst.Ident{dst.NewIdent("deadline")},
		Type:  &dst.Ident{Path: path, Name: "Builder"},
	})
}

// specNames returns the names of all import specs of the given path ("" for an unaliased spec),
// in file order.
func (e *fileEnv) specNames(path string) []string {
	var names []string
	for _, spec := range e.file.Imports {
		if spec.Path.Value == `"`+path+`"` {
			name := ""
			if spec.Name != nil {
				name = spec.Name.Name
			}
			names = append(names, name)
		}
	}
	return names
}

func (e *fileEnv) requireSpec(t *testing.T, path string, wantNames ...string) {
	t.Helper()
	require.Equal(t, wantNames, e.specNames(path), "unexpected import specs for %q", path)
}

type request struct {
	file         int    // index into the scenario's file list
	joinIndex    int    // which os.Getenv call to wrap
	path         string // requested import path
	localName    string // the template's spelling of the package name
	blank        bool   // blank request: registers a blank import, no qualified reference
	closureLocal string // wrap the generated reference in a closure declaring this local first
}

type scenario struct {
	id       string
	files    []srcFile
	requests []request
	prepend  func(t *testing.T, envs []fileEnv) // extra generated code, applied before requests
	wants    []want
}

type want struct {
	file  int
	path  string
	specs []string
	refs  map[string]ReferenceKind
}

// runScenario decorates the sources, applies the requests through one ReferenceMap per file (as
// production does), then runs AddSyntheticImports per touched file.
func runScenario(t *testing.T, s scenario) ([]fileEnv, map[int]*ReferenceMap) {
	t.Helper()

	envs := decoratePackage(t, s.files)
	if s.prepend != nil {
		s.prepend(t, envs)
	}

	refMaps := make(map[int]*ReferenceMap)
	for _, req := range s.requests {
		env := envs[req.file]
		var chain []dst.Node
		if req.blank {
			chain = env.nodeChain(env.joinCall(t, req.joinIndex))
		} else {
			chain = env.simulateWrap(t, req.joinIndex, req.path, req.localName, req.closureLocal)
		}
		rm, found := refMaps[req.file]
		if !found {
			rm = new(ReferenceMap)
			*rm = NewReferenceMap(env.nodeMap, env.scopes)
			refMaps[req.file] = rm
		}
		rm.AddImport(env.file, chain, req.path, req.localName)
	}

	for _, idx := range slices.Sorted(maps.Keys(refMaps)) {
		refMaps[idx].AddSyntheticImports(envs[idx].file)
	}

	expectedRefs := make(map[int]map[string]ReferenceKind)
	for _, w := range s.wants {
		envs[w.file].requireSpec(t, w.path, w.specs...)
		if w.refs == nil {
			continue
		}
		if expectedRefs[w.file] == nil {
			expectedRefs[w.file] = make(map[string]ReferenceKind)
		}
		for path, kind := range w.refs {
			expectedRefs[w.file][path] = kind
		}
	}
	for file, expected := range expectedRefs {
		require.Equal(t, expected, refMaps[file].Map(), "%s: unexpected synthetic references", s.id)
	}
	for file, refMap := range refMaps {
		if _, found := expectedRefs[file]; !found {
			require.Empty(t, refMap.Map(), "%s: no synthetic reference expected", s.id)
		}
	}
	return envs, refMaps
}

var importStmtRefs = map[string]ReferenceKind{"strings": ImportStatement}

func TestImportAliasPlanning(t *testing.T) {
	// The common shape: an aliased strings import, an unaffected original reference, and one or
	// more os.Getenv join points.
	const header = `package test

import (
  "os"
  str "strings"
)

func helper(s string) string { return str.ToLower(s) }

`

	join := func(body string) string { return header + "func join() {\n  " + body + "\n}\n" }
	plainJoin := join("os.Getenv(\"HOME\")")

	scenarios := []scenario{
		// --- Section A: import forms and alias ownership -------------------------------
		{
			// A01: unaliased import, no shadow: reuse the actual package name, no reference.
			id: "A01/unaliased-reuse",
			files: []srcFile{src(`package test

import (
  "os"
  "strings"
)

func helper(s string) string { return strings.ToLower(s) }

func join() { os.Getenv("HOME") }
`)},
			requests: []request{{path: "strings", localName: "strings"}},
			wants:    []want{{path: "strings", specs: []string{""}}},
		},
		{
			// A03: explicit alias, no shadow: reuse the file's alias, no reference.
			id:       "A03/real-alias-reuse",
			files:    []srcFile{src(plainJoin)},
			requests: []request{{path: "strings", localName: "strings"}},
			wants:    []want{{path: "strings", specs: []string{"str"}}},
		},
		{
			// A06: template spells the real alias; reuse while it resolves to strings.
			id:       "A06/template-real-alias-reuse",
			files:    []srcFile{src(plainJoin)},
			requests: []request{{path: "strings", localName: "str"}},
			wants:    []want{{path: "strings", specs: []string{"str"}}},
		},
		{
			// A06: same, but shadowed at the join point: generate.
			id:       "A06/template-real-alias-shadowed",
			files:    []srcFile{src(join("str := \"shadowed\"\n  _ = str\n  os.Getenv(\"HOME\")"))},
			requests: []request{{path: "strings", localName: "str"}},
			wants:    []want{{path: "strings", specs: []string{"__orchestrion_str"}, refs: importStmtRefs}},
		},
		{
			// A11: user alias with the orchestrion prefix is source-owned; shadowed: generate.
			id: "A11/user-prefix-shadowed",
			files: []srcFile{src(`package test

import (
  "os"
  __orchestrion_user "strings"
)

func helper(s string) string { return __orchestrion_user.ToLower(s) }

func join() {
  __orchestrion_user := "shadowed"
  _ = __orchestrion_user
  os.Getenv("HOME")
}
`)},
			requests: []request{{path: "strings", localName: "strings"}},
			wants:    []want{{path: "strings", specs: []string{"__orchestrion_strings"}, refs: importStmtRefs}},
		},
		{
			// A12: user alias that spells exactly the generated base, unshadowed: reuse it.
			id: "A12/user-prefix-reused",
			files: []srcFile{src(`package test

import (
  "os"
  __orchestrion_strings "strings"
)

func helper(s string) string { return __orchestrion_strings.ToLower(s) }

func join() { os.Getenv("HOME") }
`)},
			requests: []request{{path: "strings", localName: "strings"}},
			wants:    []want{{path: "strings", specs: []string{"__orchestrion_strings"}}},
		},
		{
			// A14: one path under two import names, both used, no shadow: consolidate by reuse.
			id: "A14/duplicate-aliased-unaliased-reuse",
			files: []srcFile{src(`package test

import (
  "os"
  "strings"
  str "strings"
)

func helper(s string) string { return str.ToLower(s) }
func helper2(s string) string { return strings.ToUpper(s) }

func join() { os.Getenv("HOME") }
`)},
			requests: []request{{path: "strings", localName: "strings"}},
			wants:    []want{{path: "strings", specs: []string{"", "str"}}},
		},
		{
			// A15: reversed spec order, one name shadowed at the join point: generate.
			id: "A15/duplicate-path-reversed-shadowed",
			files: []srcFile{src(`package test

import (
  "os"
  str "strings"
  "strings"
)

func helper(s string) string { return str.ToLower(s) }
func helper2(s string) string { return strings.ToUpper(s) }

func join() {
  str := "shadowed"
  _ = str
  os.Getenv("HOME")
}
`)},
			requests: []request{{path: "strings", localName: "strings"}},
			wants:    []want{{path: "strings", specs: []string{"__orchestrion_strings", ""}, refs: importStmtRefs}},
		},
		{
			// A09: blank import, named request: promote to one safe named import.
			id: "A09/blank-promoted",
			files: []srcFile{src(`package test

import (
  "os"
  _ "strings"
)

func join() { os.Getenv("HOME") }
`)},
			requests: []request{{path: "strings", localName: "strings"}},
			wants:    []want{{path: "strings", specs: []string{"__orchestrion_strings"}, refs: importStmtRefs}},
		},
		{
			// A10: blank request on an already-named import: nothing to add.
			id:       "A10/blank-request-keeps-named",
			files:    []srcFile{src(plainJoin)},
			requests: []request{{path: "strings", localName: "_", blank: true}},
			wants:    []want{{path: "strings", specs: []string{"str"}}},
		},

		// --- Section B: lexical scope at the join point --------------------------------
		{
			id:       "B01/local-variable",
			files:    []srcFile{src(join("str := \"shadowed\"\n  _ = str\n  os.Getenv(\"HOME\")"))},
			requests: []request{{path: "strings", localName: "strings"}},
			wants:    []want{{path: "strings", specs: []string{"__orchestrion_strings"}, refs: importStmtRefs}},
		},
		{
			id:       "B01/local-constant",
			files:    []srcFile{src(join("const str = \"shadowed\"\n  _ = str\n  os.Getenv(\"HOME\")"))},
			requests: []request{{path: "strings", localName: "strings"}},
			wants:    []want{{path: "strings", specs: []string{"__orchestrion_strings"}, refs: importStmtRefs}},
		},
		{
			id:       "B02/parameter",
			files:    []srcFile{src(header + "func join(str string) { os.Getenv(\"HOME\") }")},
			requests: []request{{path: "strings", localName: "strings"}},
			wants:    []want{{path: "strings", specs: []string{"__orchestrion_strings"}, refs: importStmtRefs}},
		},
		{
			id:       "B03/named-result",
			files:    []srcFile{src(header + "func join() (str string) {\n  os.Getenv(\"HOME\")\n  return\n}")},
			requests: []request{{path: "strings", localName: "strings"}},
			wants:    []want{{path: "strings", specs: []string{"__orchestrion_strings"}, refs: importStmtRefs}},
		},
		{
			id: "B04/receiver",
			files: []srcFile{src(header + `type holder struct{}

func (str holder) join() { os.Getenv("HOME") }
`)},
			requests: []request{{path: "strings", localName: "strings"}},
			wants:    []want{{path: "strings", specs: []string{"__orchestrion_strings"}, refs: importStmtRefs}},
		},
		{
			id:       "B05/type-parameter",
			files:    []srcFile{src(header + "func join[str any]() { os.Getenv(\"HOME\") }")},
			requests: []request{{path: "strings", localName: "strings"}},
			wants:    []want{{path: "strings", specs: []string{"__orchestrion_strings"}, refs: importStmtRefs}},
		},
		{
			id: "B06/receiver-type-parameter",
			files: []srcFile{src(header + `type box[T any] struct{}

func (v box[str]) join() { os.Getenv("HOME") }
`)},
			requests: []request{{path: "strings", localName: "strings"}},
			wants:    []want{{path: "strings", specs: []string{"__orchestrion_strings"}, refs: importStmtRefs}},
		},
		{
			id:       "B07/range-body",
			files:    []srcFile{src(join("for str := range []int{1} {\n    _ = str\n    os.Getenv(\"HOME\")\n  }"))},
			requests: []request{{path: "strings", localName: "strings"}},
			wants:    []want{{path: "strings", specs: []string{"__orchestrion_strings"}, refs: importStmtRefs}},
		},
		{
			// B08: the range variable is not in scope in the range expression: reuse.
			id:       "B08/range-expression",
			files:    []srcFile{src(join("for _, str := range []string{os.Getenv(\"HOME\")} {\n    _ = str\n  }"))},
			requests: []request{{path: "strings", localName: "strings"}},
			wants:    []want{{path: "strings", specs: []string{"str"}}},
		},
		{
			id:       "B09/if-init",
			files:    []srcFile{src(join("if str := \"shadowed\"; str != \"\" {\n    os.Getenv(\"HOME\")\n  }"))},
			requests: []request{{path: "strings", localName: "strings"}},
			wants:    []want{{path: "strings", specs: []string{"__orchestrion_strings"}, refs: importStmtRefs}},
		},
		{
			id:       "B10/switch-init",
			files:    []srcFile{src(join("switch str := \"shadowed\"; str {\n  case \"shadowed\":\n    os.Getenv(\"HOME\")\n  }"))},
			requests: []request{{path: "strings", localName: "strings"}},
			wants:    []want{{path: "strings", specs: []string{"__orchestrion_strings"}, refs: importStmtRefs}},
		},
		{
			id:       "B11/type-switch",
			files:    []srcFile{src(header + "func join() string {\n  switch str := any(\"shadowed\").(type) {\n  case string:\n    return str + os.Getenv(\"HOME\")\n  }\n  return \"\"\n}")},
			requests: []request{{path: "strings", localName: "strings"}},
			wants:    []want{{path: "strings", specs: []string{"__orchestrion_strings"}, refs: importStmtRefs}},
		},
		{
			id:       "B12/for-init",
			files:    []srcFile{src(join("for str := 0; str < 1; str++ {\n    os.Getenv(\"HOME\")\n  }"))},
			requests: []request{{path: "strings", localName: "strings"}},
			wants:    []want{{path: "strings", specs: []string{"__orchestrion_strings"}, refs: importStmtRefs}},
		},
		{
			id:       "B12/select-receive",
			files:    []srcFile{src(join("ch := make(chan string)\n  select {\n  case str := <-ch:\n    os.Getenv(str)\n  }"))},
			requests: []request{{path: "strings", localName: "strings"}},
			wants:    []want{{path: "strings", specs: []string{"__orchestrion_strings"}, refs: importStmtRefs}},
		},
		{
			// B13: closure parameter and captured outer variable both shadow the alias.
			id: "B13/closure",
			files: []srcFile{src(header + `func join() string {
  str := "outer"
  _ = str
  f := func(str string) string {
    os.Getenv("HOME")
    return str
  }
  return f(str)
}
`)},
			requests: []request{{path: "strings", localName: "strings"}},
			wants:    []want{{path: "strings", specs: []string{"__orchestrion_strings"}, refs: importStmtRefs}},
		},
		{
			// B14: labels are a separate namespace: they never shadow an import name.
			id:       "B14/label-named-alias",
			files:    []srcFile{src(join("str:\n  for {\n    break str\n  }\n  os.Getenv(\"HOME\")"))},
			requests: []request{{path: "strings", localName: "strings"}},
			wants:    []want{{path: "strings", specs: []string{"str"}}},
		},
		{
			// B14: a label spelled like the generated base must not force a suffix either.
			id:       "B14/label-named-base",
			files:    []srcFile{src(join("str := \"shadowed\"\n  _ = str\n  __orchestrion_strings:\n  for {\n    break __orchestrion_strings\n  }\n  os.Getenv(\"HOME\")"))},
			requests: []request{{path: "strings", localName: "strings"}},
			wants:    []want{{path: "strings", specs: []string{"__orchestrion_strings"}, refs: importStmtRefs}},
		},
		{
			// B15: a short declaration does not cover its own right-hand side: reuse.
			id:       "B15/short-decl-rhs",
			files:    []srcFile{src(header + "func join() string {\n  str := os.Getenv(\"HOME\")\n  return str\n}")},
			requests: []request{{path: "strings", localName: "strings"}},
			wants:    []want{{path: "strings", specs: []string{"str"}}},
		},
		{
			// B16: an outer local does shadow the alias on the initializer's right-hand side.
			id:       "B16/outer-binding-at-rhs",
			files:    []srcFile{src(header + "func join() string {\n  var str string\n  str2 := os.Getenv(\"HOME\")\n  return str + str2\n}")},
			requests: []request{{path: "strings", localName: "strings"}},
			wants:    []want{{path: "strings", specs: []string{"__orchestrion_strings"}, refs: importStmtRefs}},
		},
		{
			// B17: a local type declaration also shadows the package name.
			id:       "B17/local-type",
			files:    []srcFile{src(join("type str struct{}\n  var x str\n  _ = x\n  os.Getenv(\"HOME\")"))},
			requests: []request{{path: "strings", localName: "strings"}},
			wants:    []want{{path: "strings", specs: []string{"__orchestrion_strings"}, refs: importStmtRefs}},
		},

		// --- Section C: whole-file safety and collisions --------------------------------
		{
			// C01: the generated base name is taken at the join point itself.
			id:       "C01/generated-collision-at-join",
			files:    []srcFile{src(join("str := \"shadowed\"\n  __orchestrion_strings := 1\n  _, _ = str, __orchestrion_strings\n  os.Getenv(\"HOME\")"))},
			requests: []request{{path: "strings", localName: "strings"}},
			wants:    []want{{path: "strings", specs: []string{"__orchestrion_strings1"}, refs: importStmtRefs}},
		},
		{
			// C02: the generated base is taken at an original reference in another function.
			id: "C02/generated-collision-original-ref",
			files: []srcFile{src(header + `func helper2(s string) string {
  __orchestrion_strings := 1
  _ = __orchestrion_strings
  return str.ToLower(s)
}

func join() {
  str := "shadowed"
  _ = str
  os.Getenv("HOME")
}
`)},
			requests: []request{{path: "strings", localName: "strings"}},
			wants:    []want{{path: "strings", specs: []string{"__orchestrion_strings1"}, refs: importStmtRefs}},
		},
		{
			// C04: the base collides at both an original and a generated reference.
			id: "C04/generated-collision-both",
			files: []srcFile{src(header + `func helper2(s string) string {
  __orchestrion_strings := 1
  _ = __orchestrion_strings
  return str.ToLower(s)
}

func join() {
  str := "shadowed"
  __orchestrion_strings := 1
  _, _ = str, __orchestrion_strings
  os.Getenv("HOME")
}
`)},
			requests: []request{{path: "strings", localName: "strings"}},
			wants:    []want{{path: "strings", specs: []string{"__orchestrion_strings1"}, refs: importStmtRefs}},
		},
		{
			// C05: a local of the base name in a scope with no reference to the path must not
			// reject the base.
			id: "C05/generated-name-unrelated-scope",
			files: []srcFile{src(header + `func helper2() int {
  __orchestrion_strings := 1
  return __orchestrion_strings
}

func join() {
  str := "shadowed"
  _ = str
  os.Getenv("HOME")
}
`)},
			requests: []request{{path: "strings", localName: "strings"}},
			wants:    []want{{path: "strings", specs: []string{"__orchestrion_strings"}, refs: importStmtRefs}},
		},
		{
			// C06: the source alias is shadowed only in an unrelated function: reuse.
			id: "C06/source-shadow-unrelated",
			files: []srcFile{src(header + `func helper2() string {
  str := "shadowed"
  _ = str
  return "x"
}

func join() { os.Getenv("HOME") }
`)},
			requests: []request{{path: "strings", localName: "strings"}},
			wants:    []want{{path: "strings", specs: []string{"str"}}},
		},
		{
			// C07: consecutive candidates collide at different references; the first free
			// deterministic suffix is chosen.
			id: "C07/generated-suffix-chain",
			files: []srcFile{src(header + `func helper2(s string) string {
  __orchestrion_strings := 1
  _ = __orchestrion_strings
  return str.ToLower(s)
}

func helper3(s string) string {
  __orchestrion_strings1 := 1
  _ = __orchestrion_strings1
  return str.ToLower(s)
}

func join() {
  str := "shadowed"
  _ = str
  os.Getenv("HOME")
}
`)},
			requests: []request{{path: "strings", localName: "strings"}},
			wants:    []want{{path: "strings", specs: []string{"__orchestrion_strings2"}, refs: importStmtRefs}},
		},
		{
			// C08: a later join point invalidates the base candidate for the whole file.
			id: "C08/alias-late-collision",
			files: []srcFile{src(header + `func joinA() {
  str := "shadowed"
  _ = str
  os.Getenv("HOME")
}

func joinB() {
  str := "shadowed"
  __orchestrion_strings := 1
  _, _ = str, __orchestrion_strings
  os.Getenv("HOME")
}
`)},
			requests: []request{
				{path: "strings", localName: "strings"},
				{joinIndex: 1, path: "strings", localName: "strings"},
			},
			wants: []want{{path: "strings", specs: []string{"__orchestrion_strings1"}, refs: importStmtRefs}},
		},
		{
			// C10: the base would compile via a local method with the member's name; it must
			// still be rejected because the rewritten original reference would silently bind to
			// the local type.
			id: "C10/generated-collision-silent-original",
			files: []srcFile{src(header + `type local struct{}

func (local) ToUpper(s string) string { return s }

func helper2(s string) string {
  __orchestrion_strings := local{}
  _ = __orchestrion_strings
  return str.ToUpper(s)
}

func join() {
  str := "shadowed"
  _ = str
  os.Getenv("HOME")
}
`)},
			requests: []request{{path: "strings", localName: "strings"}},
			wants:    []want{{path: "strings", specs: []string{"__orchestrion_strings1"}, refs: importStmtRefs}},
		},
		{
			// C11: generated lexical scopes (a template-declared local around its own package
			// use) count as collisions.
			id:       "C11/generated-scope-collision",
			files:    []srcFile{src(join("str := \"shadowed\"\n  _ = str\n  os.Getenv(\"HOME\")"))},
			requests: []request{{path: "strings", localName: "strings", closureLocal: "__orchestrion_strings"}},
			wants:    []want{{path: "strings", specs: []string{"__orchestrion_strings1"}, refs: importStmtRefs}},
		},

		// --- Section D and E: path identity, requests, determinism ----------------------
		{
			// D02/E03: one path, two template spellings, several join points: one stable alias
			// built from the first spelling.
			id: "D02/one-path-two-spellings",
			files: []srcFile{src(header + `func joinA() { os.Getenv("HOME") }

func joinB() {
  str := "shadowed"
  _ = str
  os.Getenv("HOME")
}
`)},
			requests: []request{
				{path: "strings", localName: "strings"},
				{joinIndex: 1, path: "strings", localName: "strs"},
			},
			wants: []want{{path: "strings", specs: []string{"__orchestrion_strings"}, refs: importStmtRefs}},
		},
		{
			// D03: several join points with mixed source-alias availability: one file-wide alias.
			id: "D03/multiple-join-points",
			files: []srcFile{src(header + `func joinA() { os.Getenv("HOME") }

func joinB() {
  str := "shadowed"
  _ = str
  os.Getenv("HOME")
}
`)},
			requests: []request{
				{path: "strings", localName: "strings"},
				{joinIndex: 1, path: "strings", localName: "strings"},
			},
			wants: []want{{path: "strings", specs: []string{"__orchestrion_strings"}, refs: importStmtRefs}},
		},
		{
			// D05: a package-level declaration of the base name in the same file forces a suffix.
			id: "D05/package-decl-same-file",
			files: []srcFile{src(header + `var __orchestrion_strings int

func join() {
  str := "shadowed"
  _ = str
  os.Getenv("HOME")
}
`)},
			requests: []request{{path: "strings", localName: "strings"}},
			wants:    []want{{path: "strings", specs: []string{"__orchestrion_strings1"}, refs: importStmtRefs}},
		},
		{
			// D05 reuse control: same declaration, but the source alias is safe: reuse.
			id: "D05/package-decl-reuse-control",
			files: []srcFile{src(header + `var __orchestrion_strings int

func join() { os.Getenv("HOME") }
`)},
			requests: []request{{path: "strings", localName: "strings"}},
			wants:    []want{{path: "strings", specs: []string{"str"}}},
		},
		{
			// D06: the package-level declaration of the base name lives in another file of the
			// package: it still counts as taken.
			id: "D06/package-decl-other-file",
			files: []srcFile{
				src(join("str := \"shadowed\"\n  _ = str\n  os.Getenv(\"HOME\")")),
				extra("package test\n\nvar __orchestrion_strings int\n"),
			},
			requests: []request{{path: "strings", localName: "strings"}},
			wants:    []want{{path: "strings", specs: []string{"__orchestrion_strings1"}, refs: importStmtRefs}},
		},
		{
			// D08: alias state is per file; a conflict in one file must not leak into the other.
			id: "D08/per-file-alias-state",
			files: []srcFile{
				src(join("str := \"shadowed\"\n  _ = str\n  os.Getenv(\"HOME\")")),
				extra(`package test

import (
  "os"
  str "strings"
)

func helper2(s string) string { return str.ToLower(s) }

func join2() {
  str := "shadowed"
  __orchestrion_strings := 1
  _, _ = str, __orchestrion_strings
  os.Getenv("HOME")
}
`),
			},
			requests: []request{
				{path: "strings", localName: "strings"},
				{file: 1, path: "strings", localName: "strings"},
			},
			wants: []want{
				{path: "strings", specs: []string{"__orchestrion_strings"}, refs: importStmtRefs},
				{file: 1, path: "strings", specs: []string{"__orchestrion_strings1"}, refs: importStmtRefs},
			},
		},
		{
			// D04: an import that exists only in another file of the package is not in scope:
			// the target file gets its own import.
			id: "D04/file-import-scope",
			files: []srcFile{
				src(`package test

import "os"

func join() { os.Getenv("HOME") }
`),
				extra(`package test

import "strings"

var _ = strings.ToLower
`),
			},
			requests: []request{{path: "strings", localName: "strings"}},
			wants:    []want{{path: "strings", specs: []string{"__orchestrion_strings"}, refs: importStmtRefs}},
		},
		{
			// D13: the real package name (math/rand/v2 is rand) is resolved from type
			// information, not from the template spelling: reuse.
			id: "D13/stdlib-versioned-name",
			files: []srcFile{src(`package test

import (
  "math/rand/v2"
  "os"
)

func helper() int { return rand.IntN(3) }

func join() { os.Getenv("HOME") }
`)},
			requests: []request{{path: "math/rand/v2", localName: "rnd"}},
			wants:    []want{{path: "math/rand/v2", specs: []string{""}}},
		},
		{
			// E01: which spec is selected is decided by path ownership, not by spec order.
			id: "E01/generated-import-ownership-order-1",
			files: []srcFile{src(`package test

import (
  "os"
  "strings"
  str "strings"
)

func helper(s string) string { return str.ToLower(s) }
func helper2(s string) string { return strings.ToUpper(s) }

func join() {
  str := "shadowed"
  _ = str
  os.Getenv("HOME")
}
`)},
			requests: []request{{path: "strings", localName: "strings"}},
			wants:    []want{{path: "strings", specs: []string{"", "__orchestrion_strings"}, refs: importStmtRefs}},
		},
		{
			id: "E01/generated-import-ownership-order-2",
			files: []srcFile{src(`package test

import (
  "os"
  str "strings"
  "strings"
)

func helper(s string) string { return str.ToLower(s) }
func helper2(s string) string { return strings.ToUpper(s) }

func join() {
  str := "shadowed"
  _ = str
  os.Getenv("HOME")
}
`)},
			requests: []request{{path: "strings", localName: "strings"}},
			wants:    []want{{path: "strings", specs: []string{"__orchestrion_strings", ""}, refs: importStmtRefs}},
		},
		{
			// E02: ownership is explicit, never the __orchestrion_ prefix; a user alias with the
			// prefix is not the generated alias, and a shadowed candidate is suffixed.
			id: "E02/ownership-not-prefix",
			files: []srcFile{src(`package test

import (
  "os"
  __orchestrion_strings "strings"
)

func helper(s string) string { return __orchestrion_strings.ToLower(s) }

func join() {
  __orchestrion_strings := 1
  _ = __orchestrion_strings
  os.Getenv("HOME")
}
`)},
			requests: []request{{path: "strings", localName: "strings"}},
			wants:    []want{{path: "strings", specs: []string{"__orchestrion_strings1"}, refs: importStmtRefs}},
		},
		{
			// E04: a blank request must not erase a named alias, in either order.
			id: "E04/blank-then-named",
			files: []srcFile{src(`package test

import "os"

func join() { os.Getenv("HOME") }
`)},
			requests: []request{
				{path: "strings", localName: "_", blank: true},
				{path: "strings", localName: "strings"},
			},
			wants: []want{{path: "strings", specs: []string{"__orchestrion_strings"}, refs: importStmtRefs}},
		},
		{
			id: "E04/named-then-blank",
			files: []srcFile{src(`package test

import "os"

func join() { os.Getenv("HOME") }
`)},
			requests: []request{
				{path: "strings", localName: "strings"},
				{path: "strings", localName: "_", blank: true},
			},
			wants: []want{{path: "strings", specs: []string{"__orchestrion_strings"}, refs: importStmtRefs}},
		},
		{
			// E05: two generated aliases competing for the same base get distinct names by path
			// order, independent of request order.
			id: "E05/alias-allocation-determinism-order-1",
			files: []srcFile{src(`package test

import (
  "math"
  "os"
  str "strings"
)

func helper(s string) string { return str.ToLower(s) }
func helperM() float64 { return math.Floor(1.5) }

func joinA() {
  str := "shadowed"
  _ = str
  os.Getenv("HOME")
}

func joinB() {
  math := float64(1)
  _ = math
  os.Getenv("HOME")
}
`)},
			requests: []request{
				{path: "strings", localName: "strings"},
				{joinIndex: 1, path: "math", localName: "strings"},
			},
			wants: []want{
				{path: "math", specs: []string{"__orchestrion_strings"}, refs: map[string]ReferenceKind{"math": ImportStatement}},
				{path: "strings", specs: []string{"__orchestrion_strings1"}, refs: map[string]ReferenceKind{"strings": ImportStatement}},
			},
		},
		{
			id: "E05/alias-allocation-determinism-order-2",
			files: []srcFile{src(`package test

import (
  "math"
  "os"
  str "strings"
)

func helper(s string) string { return str.ToLower(s) }
func helperM() float64 { return math.Floor(1.5) }

func joinA() {
  str := "shadowed"
  _ = str
  os.Getenv("HOME")
}

func joinB() {
  math := float64(1)
  _ = math
  os.Getenv("HOME")
}
`)},
			requests: []request{
				{joinIndex: 1, path: "math", localName: "strings"},
				{path: "strings", localName: "strings"},
			},
			wants: []want{
				{path: "math", specs: []string{"__orchestrion_strings"}, refs: map[string]ReferenceKind{"math": ImportStatement}},
				{path: "strings", specs: []string{"__orchestrion_strings1"}, refs: map[string]ReferenceKind{"strings": ImportStatement}},
			},
		},
		{
			// P1: the type parameters of a generic type declaration scope the whole
			// declaration, so a generated field type inside it is shadowed by a
			// same-named type parameter.
			id: "P1/generic-type-spec-type-param",
			files: []srcFile{src(`package test

import (
  "os"
  str "strings"
)

type box[str any] struct {
  existing int
}

func helper(s string) string { return str.ToLower(s) }

func join() { os.Getenv("HOME") }
`)},
			requests: []request{{path: "strings", localName: "strings"}},
			prepend: func(t *testing.T, envs []fileEnv) {
				envs[0].simulateAddField(t, "box", "strings")
			},
			wants: []want{{path: "strings", specs: []string{"__orchestrion_strings"}, refs: importStmtRefs}},
		},
		{
			// P2: a type-switch guard variable scopes each clause body only, so a
			// reference in a case type is not shadowed by the guard.
			id: "P2/type-switch-case-type-position",
			files: []srcFile{src(`package test

import (
  "os"
  str "strings"
)

func helper(s string) string { return str.ToLower(s) }

func join() string {
  switch str := any("x").(type) {
  case *str.Builder:
    _ = str
  }
  return os.Getenv("HOME")
}
`)},
			requests: []request{{path: "strings", localName: "strings"}},
			wants:    []want{{path: "strings", specs: []string{"str"}}},
		},
		{
			// P2: a case-clause expression precedes the body declarations, so a body
			// declaration does not shadow it.
			id: "P2/case-expression-before-body-decl",
			files: []srcFile{src(`package test

import (
  "os"
  str "strings"
)

func helper(s string) string { return str.ToLower(s) }

func join() string {
  switch "a" {
  case str.ToLower("a"):
    str := "local"
    _ = str
  }
  return os.Getenv("HOME")
}
`)},
			requests: []request{{path: "strings", localName: "strings"}},
			wants:    []want{{path: "strings", specs: []string{"str"}}},
		},
		{
			// A13: a source alias that spells the generated base for a different path is
			// source-owned; the generated candidate for this path is suffixed instead. The other
			// path's import is aliased to the base spelling.
			id: "A13/user-prefix-other-path",
			files: []srcFile{src(`package test

import (
  "os"
  __orchestrion_strings "math"
  str "strings"
)

func helper(s string) string { return str.ToLower(s) }
func helperM() float64 { return __orchestrion_strings.Floor(1.5) }

func join() {
  str := "shadowed"
  _ = str
  os.Getenv("HOME")
}
`)},
			requests: []request{{path: "strings", localName: "strings"}},
			wants: []want{
				{path: "math", specs: []string{"__orchestrion_strings"}},
				{path: "strings", specs: []string{"__orchestrion_strings1"}, refs: importStmtRefs},
			},
		},
	}

	for _, s := range scenarios {
		t.Run(s.id, func(t *testing.T) {
			runScenario(t, s)
		})
	}
}

// TestGeneratedAliasShape (R-new) validates the identifier shape of every alias the planner
// generates: a valid Go identifier that is not exported, not blank or dot, and not predeclared.
func TestGeneratedAliasShape(t *testing.T) {
	const header = `package test

import (
  "os"
  str "strings"
)

func helper(s string) string { return str.ToLower(s) }

`

	for _, tc := range []struct {
		name     string
		scenario scenario
		alias    string
	}{
		{
			name: "base",
			scenario: scenario{
				files:    []srcFile{src(header + "func join() {\n  str := \"shadowed\"\n  _ = str\n  os.Getenv(\"HOME\")\n}")},
				requests: []request{{path: "strings", localName: "strings"}},
				wants:    []want{{path: "strings", specs: []string{"__orchestrion_strings"}, refs: importStmtRefs}},
			},
			alias: "__orchestrion_strings",
		},
		{
			name: "first-suffix",
			scenario: scenario{
				files:    []srcFile{src(header + "func join() {\n  str := \"shadowed\"\n  __orchestrion_strings := 1\n  _, _ = str, __orchestrion_strings\n  os.Getenv(\"HOME\")\n}")},
				requests: []request{{path: "strings", localName: "strings"}},
				wants:    []want{{path: "strings", specs: []string{"__orchestrion_strings1"}, refs: importStmtRefs}},
			},
			alias: "__orchestrion_strings1",
		},
		{
			name: "second-suffix",
			scenario: scenario{
				files: []srcFile{src(header + `func helper2(s string) string {
  __orchestrion_strings := 1
  _ = __orchestrion_strings
  return str.ToLower(s)
}

func helper3(s string) string {
  __orchestrion_strings1 := 1
  _ = __orchestrion_strings1
  return str.ToLower(s)
}

func join() {
  str := "shadowed"
  _ = str
  os.Getenv("HOME")
}
`)},
				requests: []request{{path: "strings", localName: "strings"}},
				wants:    []want{{path: "strings", specs: []string{"__orchestrion_strings2"}, refs: importStmtRefs}},
			},
			alias: "__orchestrion_strings2",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			envs, _ := runScenario(t, tc.scenario)
			alias := tc.alias
			require.True(t, token.IsIdentifier(alias), "%q must be a valid identifier", alias)
			require.False(t, token.IsExported(alias), "%q must not be exported", alias)
			require.NotEqual(t, "_", alias)
			require.NotEqual(t, ".", alias)
			require.Nil(t, types.Universe.Lookup(alias), "%q must not be predeclared", alias)
			for _, env := range envs {
				for _, spec := range env.file.Imports {
					if spec.Name != nil {
						name := spec.Name.Name
						if len(name) > 0 && name != "_" && name != "." {
							require.False(t, token.IsExported(name), "import name %q must not be exported", name)
						}
					}
				}
			}
		})
	}
}

// TestGeneratedLocalShadowsOriginalRef (F04a) covers generated code that declares a local with the
// real alias spelling before an original reference: reuse is not safe, the planner must generate.
func TestGeneratedLocalShadowsOriginalRef(t *testing.T) {
	scenario := scenario{
		id: "F04/generated-local-shadows-original-ref",
		files: []srcFile{src(`package test

import (
  "os"
  str "strings"
)

func helper(s string) string { return str.ToLower(s) }

func join() { os.Getenv("HOME") }
`)},
		prepend: func(t *testing.T, envs []fileEnv) {
			envs[0].simulatePrepend(t, "helper", "str")
		},
		requests: []request{{path: "strings", localName: "strings"}},
		wants:    []want{{path: "strings", specs: []string{"__orchestrion_strings"}, refs: importStmtRefs}},
	}
	runScenario(t, scenario)
}
