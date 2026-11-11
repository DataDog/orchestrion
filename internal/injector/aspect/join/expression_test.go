// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package join

import (
	"go/ast"
	"go/types"
	"testing"

	"github.com/DataDog/orchestrion/internal/injector/aspect/context"
	"github.com/DataDog/orchestrion/internal/injector/aspect/context/contexttest"
	"github.com/DataDog/orchestrion/internal/injector/aspect/may"
	"github.com/dave/dst"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testImportPath = "example.com/jointest"

// typeArgsDecls declares the generic function family used by the explicit
// instantiation tests: one, two, and four declared type parameters, plus the
// supporting named types. These declarations carry no imports, so they can be
// inserted after the import block of any test source.
const typeArgsDecls = `
type Box[T any] struct{ Value T }
type Holder[T any] struct{ Fn func(T) T }
type Wrap[T any] []T
type Callback[T any] func(T) T

func F[A any](a A) A                          { return a }
func G[A, B any](a A, b B) B                 { var z B; return z }
func H[A, B, C, D any](a A, b B, c C, d D) D { var z D; return z }
func Plain(a int) int                         { return a }
`

// buildSrc assembles a complete test source from an optional import block and
// the declarations and code of a case.
func buildSrc(imports string, src string) string {
	if imports == "" {
		return "package test\n" + typeArgsDecls + src
	}
	return "package test\nimport (\n" + imports + "\n)" + typeArgsDecls + src
}

// splitSymbol splits a "path.Name" or "Name" join point symbol the same way
// the YAML unmarshaler does.
func splitSymbol(t *testing.T, symbol string) (path string, name string) {
	t.Helper()

	matches := funcNamePattern.FindStringSubmatch(symbol)
	require.NotNil(t, matches, "invalid function name %q", symbol)
	return matches[1], matches[2]
}

// matchResults evaluates the join point against every call expression of the
// fixture and maps the rendered callee of each call to whether it matched.
func matchResults(t *testing.T, fx *contexttest.Fixture, point Point) map[string]bool {
	t.Helper()

	results := make(map[string]bool)
	err := fx.Walk(func(ctx context.AspectContext) error {
		call, ok := ctx.Node().(*dst.CallExpr)
		if !ok {
			return nil
		}
		results[contexttest.RenderExpr(call.Fun)] = point.Matches(ctx)
		return nil
	})
	require.NoError(t, err)
	return results
}

// assertMatches checks the expected callee -> match mapping. Every expected
// callee must occur in the source, so a typo in an expectation fails the test
// instead of passing vacuously.
func assertMatches(t *testing.T, fx *contexttest.Fixture, point Point, symbol string, expected map[string]bool) {
	t.Helper()

	results := matchResults(t, fx, point)
	for callee, want := range expected {
		_, seen := results[callee]
		require.True(t, seen, "no call with callee %q found in the fixture source", callee)
		assert.Equal(t, want, results[callee], "unexpected match result for callee %q (target %q)", callee, symbol)
	}
}

type matchCase struct {
	name string
	// symbol is the function-call join point target, e.g. "F" or "slices.Clone".
	symbol string
	// imports is the import block of the source, without the "import" keyword
	// framing when it holds a single spec; e.g. "\"slices\"" or the body of a
	// parenthesized import block.
	imports string
	// extra declares synthetic external packages (path -> source).
	extra map[string]string
	// src is the declarations and code of the package under test. The package
	// clause is "test"; the generic family above is always declared.
	src      string
	expected map[string]bool
}

func (tc matchCase) run(t *testing.T) {
	t.Helper()

	path, name := splitSymbol(t, tc.symbol)
	fx := contexttest.New(t, testImportPath, buildSrc(tc.imports, tc.src), tc.extra)
	assertMatches(t, fx, FunctionCall(path, name), tc.symbol, tc.expected)
}

// TestFunctionCallMatchesTypeArgs covers direct calls with explicit type
// arguments: every qualification form, every supplied arity, and target
// identity (matrix C01-C13, C16, S01-S03).
func TestFunctionCallMatchesTypeArgs(t *testing.T) {
	for _, tc := range []matchCase{
		{
			name:   "same-package one type argument",
			symbol: "F",
			src: `
func Use() int {
	return F[int](1)
}
`,
			expected: map[string]bool{"F[int]": true},
		},
		{
			name:   "same-package two type arguments",
			symbol: "G",
			src: `
func Use() string {
	return G[int, string](1, "x")
}
`,
			expected: map[string]bool{"G[int, string]": true},
		},
		{
			name:   "same-package four type arguments",
			symbol: "H",
			src: `
func Use() float64 {
	return H[int, string, bool, float64](1, "x", true, 2.0)
}
`,
			expected: map[string]bool{"H[int, string, bool, float64]": true},
		},
		{
			name:   "partial instantiation infers the remainder",
			symbol: "G",
			src: `
func Use() string {
	return G[int](1, "x")
}
`,
			expected: map[string]bool{"G[int]": true},
		},
		{
			name:   "partial instantiation of four parameters",
			symbol: "H",
			src: `
func Use() float64 {
	return H[int, string](1, "x", true, 2.0)
}
`,
			expected: map[string]bool{"H[int, string]": true},
		},
		{
			name:   "complex type argument expressions",
			symbol: "F",
			src: `
func Use() {
	_ = F[[]int](nil)
	_ = F[map[string]int](nil)
	_ = F[[2]int]([2]int{})
	_ = F[*int](nil)
	_ = F[func(int) int](nil)
	_ = F[Box[int]](Box[int]{})
}
`,
			expected: map[string]bool{
				"F[[]int]":           true,
				"F[map[string]int]":  true,
				"F[[2]int]":          true,
				"F[*int]":            true,
				"F[<*dst.FuncType>]": true,
				"F[Box[int]]":        true,
			},
		},
		{
			name:   "caller type parameter as type argument",
			symbol: "F",
			src: `
func Caller[T any](v T) T {
	return F[T](v)
}
`,
			expected: map[string]bool{"F[T]": true},
		},
		{
			name:    "imported qualified call",
			symbol:  "slices.Clone",
			imports: "\"slices\"",
			src: `
func Use(xs []int) []int {
	_ = slices.Clone[[]int](xs)
	return slices.Clone[[]int, int](xs)
}
`,
			expected: map[string]bool{
				"slices.Clone[[]int]":      true,
				"slices.Clone[[]int, int]": true,
			},
		},
		{
			name:    "aliased import call",
			symbol:  "slices.Clone",
			imports: "s \"slices\"",
			src: `
func Use(xs []int) []int {
	return s.Clone[[]int](xs)
}
`,
			expected: map[string]bool{"slices.Clone[[]int]": true},
		},
		{
			name:    "dot-imported call",
			symbol:  "slices.Clone",
			imports: ". \"slices\"",
			src: `
func Use(xs []int) []int {
	return Clone[[]int](xs)
}
`,
			expected: map[string]bool{"slices.Clone[[]int]": true},
		},
		{
			name:   "inferred call keeps matching",
			symbol: "F",
			src: `
func Use() int {
	return F(1)
}
`,
			expected: map[string]bool{"F": true},
		},
		{
			name:   "non-generic call keeps matching",
			symbol: "Plain",
			src: `
func Use() int {
	return Plain(1)
}
`,
			expected: map[string]bool{"Plain": true},
		},
		{
			name:   "different name never matches",
			symbol: "F",
			src: `
func Use() string {
	return G[int](1, "x")
}
`,
			expected: map[string]bool{"G[int]": false},
		},
		{
			name:   "different package never matches",
			symbol: "example.com/other.F",
			src: `
func Use() int {
	return F[int](1)
}
`,
			expected: map[string]bool{"F[int]": false},
		},
		{
			name:    "local function with the target name never matches an imported target",
			symbol:  "slices.Clone",
			imports: "\"slices\"",
			src: `
func Clone[T any](v T) T { return v }

func Use(xs []int) []int {
	_ = Clone[int](1)
	return slices.Clone[[]int](xs)
}
`,
			expected: map[string]bool{
				"Clone[int]":          false,
				"slices.Clone[[]int]": true,
			},
		},
		{
			name:   "local slice shadowing the target name does not match",
			symbol: "F",
			src: `
func Use() int {
	F := []func(int) int{func(int) int { return 0 }}
	return F[0](1)
}
`,
			expected: map[string]bool{"F[0]": false},
		},
		{
			name:    "local map shadowing a dot-imported name does not match",
			symbol:  "slices.Clone",
			imports: ". \"slices\"",
			src: `
var _ = Clone[[]int]([]int{})

func Use() int {
	Clone := map[string]func() int{"x": func() int { return 0 }}
	return Clone["x"]()
}
`,
			expected: map[string]bool{
				"slices.Clone[[]int]": true,
				"Clone[\"x\"]":        false,
			},
		},
		{
			name:    "local variable named like an imported package does not match",
			symbol:  "slices.Clone",
			imports: "\"slices\"",
			src: `
var _ = slices.Clone[[]int]([]int{})

type wrapper struct{ Clone []func() int }

func Use() int {
	slices := wrapper{Clone: []func() int{func() int { return 0 }}}
	return slices.Clone[0]()
}
`,
			expected: map[string]bool{
				"slices.Clone[[]int]": true,
				"slices.Clone[0]":     false,
			},
		},
		{
			name:    "generic call matches like its non-generic counterpart",
			symbol:  "slices.Clone",
			imports: "\"slices\"",
			src: `
func Use(xs []int) bool {
	_ = slices.Clone[[]int](xs)
	return slices.Contains(xs, 1)
}
`,
			expected: map[string]bool{
				"slices.Clone[[]int]": true,
				"slices.Contains":     false,
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) { tc.run(t) })
	}
}

// TestFunctionCallMatchesVendoredPathParity asserts that a generic call and a
// non-generic call from the same package match through the same canonical
// package path (matrix C16). Vendored modules resolve to the same import path
// at type-checking time, so parity here is what a -mod=vendor build observes.
func TestFunctionCallMatchesVendoredPathParity(t *testing.T) {
	src := buildSrc("\"slices\"", `
func Use(xs []int) bool {
	_ = slices.Clone[[]int](xs)
	return slices.Contains(xs, 1)
}
`)
	fx := contexttest.New(t, testImportPath, src, nil)

	assertMatches(t, fx, FunctionCall("slices", "Clone"), "slices.Clone", map[string]bool{
		"slices.Clone[[]int]": true,
		"slices.Contains":     false,
	})
	assertMatches(t, fx, FunctionCall("slices", "Contains"), "slices.Contains", map[string]bool{
		"slices.Contains":     true,
		"slices.Clone[[]int]": false,
	})
}

// TestFunctionCallMatchesTypeArgsBoundaries covers the callees that must stay
// outside the new explicit-instantiation matching path (matrix B01-B12, B14-B23).
// All of these must remain non-matches before and after the fix.
func TestFunctionCallMatchesTypeArgsBoundaries(t *testing.T) {
	for _, tc := range []matchCase{
		{
			name:    "stored instantiation value is not a call",
			symbol:  "F",
			imports: "\"fmt\"",
			src: `
func Use() {
	f := F[int]
	fmt.Println(f)
}
`,
			expected: map[string]bool{"fmt.Println": false},
		},
		{
			name:   "indirect call through a stored instantiation",
			symbol: "F",
			src: `
func Use() int {
	f := F[int]
	return f(1)
}
`,
			expected: map[string]bool{"f": false},
		},
		{
			name:   "parenthesized non-generic callee",
			symbol: "Plain",
			src: `
func Use() int {
	return (Plain)(1)
}
`,
			expected: map[string]bool{"(Plain)": false},
		},
		{
			name:   "parenthesized instantiated callee",
			symbol: "F",
			src: `
func Use() int {
	return (F[int])(1)
}
`,
			expected: map[string]bool{"(F[int])": false},
		},
		{
			name:   "parenthesized indexed slice of functions",
			symbol: "Handlers",
			src: `
var Handlers = []func() int{func() int { return 0 }}

func Use() int {
	return (Handlers)[0]()
}
`,
			expected: map[string]bool{"(Handlers)[0]": false},
		},
		{
			name:   "method call on a generic receiver type",
			symbol: "F",
			src: `
func (Box[T]) Get(v T) T { return v }

func Use(b Box[int]) int {
	return b.Get(1)
}
`,
			expected: map[string]bool{"b.Get": false},
		},
		{
			name:   "method expression on an instantiated type",
			symbol: "F",
			src: `
func (Box[T]) Get(v T) T { return v }

func Use(b Box[int], p *Box[int]) {
	_ = Box[int].Get(b, 1)
	_ = (*Box[int]).Get(p, 1)
}
`,
			expected: map[string]bool{
				"Box[int].Get":    false,
				"(*Box[int]).Get": false,
			},
		},
		{
			name:   "generic named type conversion",
			symbol: "Wrap",
			src: `
func Use() Wrap[int] {
	return Wrap[int]([]int{1})
}
`,
			expected: map[string]bool{"Wrap[int]": false},
		},
		{
			name:   "generic named function type conversion",
			symbol: "Callback",
			src: `
func Use() Callback[int] {
	return Callback[int](func(int) int { return 0 })
}
`,
			expected: map[string]bool{"Callback[int]": false},
		},
		{
			name:   "composite literal is not a call",
			symbol: "Box",
			src: `
func Use() Box[int] {
	return Box[int]{Value: 1}
}
`,
			expected: make(map[string]bool),
		},
		{
			name:   "field call on a composite literal result",
			symbol: "F",
			src: `
func Use() int {
	return Holder[int]{Fn: func(int) int { return 0 }}.Fn(1)
}
`,
			expected: map[string]bool{"<*dst.CompositeLit>.Fn": false},
		},
		{
			name:   "slice of functions indexed call",
			symbol: "Handlers",
			src: `
var Handlers = []func() int{func() int { return 0 }}

func Use() int {
	return Handlers[0]()
}
`,
			expected: map[string]bool{"Handlers[0]": false},
		},
		{
			name:   "array of functions indexed call",
			symbol: "Handlers",
			src: `
var Handlers = [2]func() int{func() int { return 0 }}

func Use() int {
	return Handlers[1]()
}
`,
			expected: map[string]bool{"Handlers[1]": false},
		},
		{
			name:   "pointer to array of functions indexed call",
			symbol: "Handlers",
			src: `
var handlers = [2]func() int{func() int { return 0 }}

func Use() int {
	p := &handlers
	return p[1]()
}
`,
			expected: map[string]bool{"p[1]": false},
		},
		{
			name:   "integer-keyed map of functions indexed call",
			symbol: "Table",
			src: `
var Table = map[int]func() int{0: func() int { return 0 }}

func Use() int {
	return Table[0]()
}
`,
			expected: map[string]bool{"Table[0]": false},
		},
		{
			name:   "string-keyed map of functions indexed call",
			symbol: "Table",
			src: `
var Table = map[string]func() string{"x": func() string { return "y" }}

func Use() string {
	return Table["x"]()
}
`,
			expected: map[string]bool{"Table[\"x\"]": false},
		},
		{
			name:   "side-effecting index expression",
			symbol: "Handlers",
			src: `
var Handlers = []func() int{func() int { return 0 }}

func key() int { return 0 }

func Use() int {
	return Handlers[key()]()
}
`,
			expected: map[string]bool{"Handlers[key()]": false},
		},
		{
			name:   "identifier-valued index",
			symbol: "Handlers",
			src: `
var Handlers = []func() int{func() int { return 0 }}

func Use() int {
	int := 0
	_ = int
	return Handlers[int]()
}
`,
			expected: map[string]bool{"Handlers[int]": false},
		},
		{
			name:   "qualified generic type conversion does not match",
			symbol: "example.com/genlib.Pair",
			extra: map[string]string{
				"example.com/genlib": `package genlib

type Pair[A, B any] func(A) B
type Struct[A, B any] struct {
	First  A
	Second B
}
`,
			},
			imports: "\"example.com/genlib\"",
			src: `
func Use(f func(int) string) genlib.Pair[int, string] {
	return genlib.Pair[int, string](f)
}
`,
			expected: map[string]bool{"example.com/genlib.Pair[int, string]": false},
		},
		{
			name:   "generic type alias conversion does not match",
			symbol: "AliasPair",
			src: `
type Pair[A, B any] func(A) B
type AliasPair[T any] = Pair[T, T]

func Use(f func(int) int) AliasPair[int] {
	return AliasPair[int](f)
}
`,
			expected: map[string]bool{"AliasPair[int]": false},
		},
	} {
		t.Run(tc.name, func(t *testing.T) { tc.run(t) })
	}
}

// TestFunctionCallMatchesTypeArgsNested covers indexed calls nested inside
// other calls, literals, and initializers (matrix P01-P04).
func TestFunctionCallMatchesTypeArgsNested(t *testing.T) {
	for _, tc := range []matchCase{
		{
			name:   "outer call of a function-returning instantiation stays unmatched",
			symbol: "FR",
			src: `
func FR[A any]() func(A) A { return func(v A) A { return v } }

func Use() int {
	return FR[int]()(1)
}
`,
			expected: map[string]bool{
				"FR[int]":   true,
				"FR[int]()": false,
			},
		},
		{
			name:   "call inside a function literal",
			symbol: "F",
			src: `
func Use() int {
	return func() int { return F[int](1) }()
}
`,
			expected: map[string]bool{"F[int]": true},
		},
		{
			name:   "call in a package-level initializer",
			symbol: "F",
			src: `
var value = F[int](1)
`,
			expected: map[string]bool{"F[int]": true},
		},
	} {
		t.Run(tc.name, func(t *testing.T) { tc.run(t) })
	}
}

// TestFunctionCallMatchesTypeArgsNestedCalls asserts that each call in a
// nested instantiation matches its own target exactly once (matrix P02).
func TestFunctionCallMatchesTypeArgsNestedCalls(t *testing.T) {
	fx := contexttest.New(t, testImportPath, buildSrc("", `
func Use() string {
	return F[string](G[int](1, "x"))
}
`), nil)

	assertMatches(t, fx, FunctionCall("", "F"), "F", map[string]bool{
		"F[string]": true,
		"G[int]":    false,
	})
	assertMatches(t, fx, FunctionCall("", "G"), "G", map[string]bool{
		"G[int]": true,
	})
}

// TestFunctionCallMatchesTypeArgsUnresolved covers indexed callees whose
// operand has no usable type information: missing node-map entries, missing
// type entries, and non-generic or named operands (matrix P06, P07, B10). The
// matcher must fail closed, never panic, and never fall back to syntax-only
// unwrapping.
func TestFunctionCallMatchesTypeArgsUnresolved(t *testing.T) {
	t.Run("missing node map entry", func(t *testing.T) {
		src := buildSrc("", `
func Use() int {
	return F[int](1)
}
`)
		fx := contexttest.New(t, testImportPath, src, nil)
		call := fx.FindCall(t, "F[int]")
		operand := call.Fun.(*dst.IndexExpr).X
		require.NotNil(t, fx.Decorator.Ast.Nodes[operand], "the operand must be in the node map")
		delete(fx.Decorator.Ast.Nodes, operand)

		assertMatches(t, fx, FunctionCall("", "F"), "F", map[string]bool{"F[int]": false})
	})

	t.Run("missing types and uses entries", func(t *testing.T) {
		src := buildSrc("\"slices\"", `
func Use(xs []int) []int {
	return slices.Clone[[]int](xs)
}
`)
		fx := contexttest.New(t, testImportPath, src, nil)
		call := fx.FindCall(t, "slices.Clone[[]int]")
		selector := call.Fun.(*dst.IndexExpr).X
		astSelector, ok := fx.Decorator.Ast.Nodes[selector].(*ast.SelectorExpr)
		require.True(t, ok, "the operand must map to an *ast.SelectorExpr")

		delete(fx.Info.Types, astSelector)
		delete(fx.Info.Uses, astSelector.Sel)

		assertMatches(t, fx, FunctionCall("slices", "Clone"), "slices.Clone", map[string]bool{"slices.Clone[[]int]": false})
	})

	t.Run("non-generic signature", func(t *testing.T) {
		src := buildSrc("", `
func Use() int {
	return F[int](1)
}
`)
		fx := contexttest.New(t, testImportPath, src, nil)
		call := fx.FindCall(t, "F[int]")
		operand := call.Fun.(*dst.IndexExpr).X.(*dst.Ident)
		astIdent, ok := fx.Decorator.Ast.Nodes[operand].(*ast.Ident)
		require.True(t, ok, "the operand must map to an *ast.Ident")

		// No valid Go produces an indexed call of a non-generic function, so this
		// case can only be exercised by substituting the resolved type of the
		// operand with a real, non-generic *types.Signature.
		nonGeneric := types.NewSignatureType(nil, nil, nil, types.NewTuple(), types.NewTuple(), false)
		fx.Info.Types[astIdent] = types.TypeAndValue{Type: nonGeneric}

		assertMatches(t, fx, FunctionCall("", "F"), "F", map[string]bool{"F[int]": false})
	})

	t.Run("named type with underlying signature", func(t *testing.T) {
		src := buildSrc("", `
func Use() int {
	return F[int](1)
}
`)
		fx := contexttest.New(t, testImportPath, src, nil)
		call := fx.FindCall(t, "F[int]")
		operand := call.Fun.(*dst.IndexExpr).X.(*dst.Ident)
		astIdent, ok := fx.Decorator.Ast.Nodes[operand].(*ast.Ident)
		require.True(t, ok, "the operand must map to an *ast.Ident")

		// A named function type has an underlying *types.Signature; the guard must
		// not reach through Underlying, or conversions such as Callback[int](f)
		// would be treated as generic calls.
		underlying := types.NewSignatureType(nil, nil, nil, types.NewTuple(), types.NewTuple(), false)
		named := types.NewNamed(types.NewTypeName(0, types.NewPackage(testImportPath, "test"), "Callback", nil), underlying, nil)
		fx.Info.Types[astIdent] = types.TypeAndValue{Type: named}

		assertMatches(t, fx, FunctionCall("", "F"), "F", map[string]bool{"F[int]": false})
	})
}

// TestFunctionCallTypeArgsInvalidInput covers invalid source: a generic method
// declaration, an attempted call of a struct literal, and a non-type type
// argument. All of these must be rejected by the type checker before any
// matching happens (matrix B08, B13, P08).
func TestFunctionCallTypeArgsInvalidInput(t *testing.T) {
	for name, src := range map[string]string{
		"method with type parameters": `package test

type Box[T any] struct{ Value T }

func (b Box[int]) Get[U any](v U) U { return v }
`,
		"call of a struct literal": `package test

type Box[T any] struct{ Value T }

func Use() {
	Box[int]{}()
}
`,
		"non-type type argument": `package test

func F[A any](a A) A { return a }

func Use(v int) int {
	return F[v](v)
}
`,
		"parenthesized generic function indexed": `package test

func F[A any](a A) A { return a }

func Use() int {
	return (F)[int](1)
}
`,
	} {
		t.Run(name, func(t *testing.T) {
			err := contexttest.TypeCheckError(t, testImportPath, src)
			require.Error(t, err, "invalid input must fail type-checking and never reach matching")
		})
	}
}

// TestFunctionCallTypeArgsCombinators covers one-of, all-of with import-path,
// and not combinators over explicit instantiations (matrix J01, J02, J04).
func TestFunctionCallTypeArgsCombinators(t *testing.T) {
	newFixture := func(t *testing.T) *contexttest.Fixture {
		return contexttest.New(t, testImportPath, buildSrc("", `
func Use() (int, string) {
	return F[int](1), G[int, string](1, "x")
}
`), nil)
	}

	t.Run("one-of matches each target once", func(t *testing.T) {
		point := OneOf(FunctionCall("", "F"), FunctionCall("", "G"))
		assertMatches(t, newFixture(t), point, "one-of", map[string]bool{
			"F[int]":         true,
			"G[int, string]": true,
		})
	})

	t.Run("all-of requires the caller import path", func(t *testing.T) {
		point := AllOf(FunctionCall("", "F"), ImportPath(testImportPath))
		assertMatches(t, newFixture(t), point, "all-of", map[string]bool{
			"F[int]":         true,
			"G[int, string]": false,
		})
	})

	t.Run("not negates the function call", func(t *testing.T) {
		point := Not(FunctionCall("", "F"))
		assertMatches(t, newFixture(t), point, "not", map[string]bool{
			"F[int]":         false,
			"G[int, string]": true,
		})
	})
}

// TestFunctionCallTypeArgsFunctionBodyBoundary asserts that a call node never
// matches a function-body join point, regardless of instantiation (matrix J05).
func TestFunctionCallTypeArgsFunctionBodyBoundary(t *testing.T) {
	fx := contexttest.New(t, testImportPath, buildSrc("", `
func Use() int {
	return F[int](1)
}
`), nil)
	point := FunctionBody(Function(Name("Use")))
	assertMatches(t, fx, point, "function-body", map[string]bool{"F[int]": false})
}

// TestFunctionCallTypeArgsPrefilters covers the package and file prefilters
// for every qualification form: they must not discard a true match (matrix J07).
func TestFunctionCallTypeArgsPrefilters(t *testing.T) {
	tests := []struct {
		name         string
		symbol       string
		pkgCtx       *may.PackageContext
		fileCtx      *may.FileContext
		pkgExpected  may.MatchType
		fileExpected may.MatchType
	}{
		{
			name:   "imported package present",
			symbol: "slices.Clone",
			pkgCtx: &may.PackageContext{ImportMap: map[string]string{"slices": ""}},
			fileCtx: &may.FileContext{
				PackageName: "test",
				FileContent: []byte(`package test; import "slices"; func f() { slices.Clone[[]int](nil) }`),
			},
			pkgExpected:  may.Match,
			fileExpected: may.Match,
		},
		{
			name:   "imported package absent",
			symbol: "slices.Clone",
			pkgCtx: &may.PackageContext{ImportMap: map[string]string{"other.example/foo": ""}},
			fileCtx: &may.FileContext{
				PackageName: "test",
				FileContent: []byte(`package test; import "slices"; func f() { slices.Clone[[]int](nil) }`),
			},
			pkgExpected:  may.NeverMatch,
			fileExpected: may.Match, // The file mentions the name; only the package filter rules the import out.
		},
		{
			name:   "same-package target",
			symbol: "F",
			pkgCtx: &may.PackageContext{ImportMap: make(map[string]string)},
			fileCtx: &may.FileContext{
				PackageName: "test",
				FileContent: []byte(`package test; func f() { F[int](1) }`),
			},
			pkgExpected:  may.Unknown, // A same-package target does not require any import.
			fileExpected: may.Match,
		},
		{
			name:   "file without the target name",
			symbol: "F",
			pkgCtx: &may.PackageContext{ImportMap: make(map[string]string)},
			fileCtx: &may.FileContext{
				PackageName: "test",
				FileContent: []byte(`package test; func f() { G[int](1) }`),
			},
			pkgExpected:  may.Unknown,
			fileExpected: may.NeverMatch,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			path, name := splitSymbol(t, tc.symbol)
			point := FunctionCall(path, name)

			assert.Equal(t, tc.pkgExpected, point.PackageMayMatch(tc.pkgCtx), "unexpected package prefilter result")
			assert.Equal(t, tc.fileExpected, point.FileMayMatch(tc.fileCtx), "unexpected file prefilter result")
		})
	}
}
