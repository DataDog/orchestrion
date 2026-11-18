// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package advice_test

import (
	"fmt"
	goast "go/ast"
	"go/importer"
	goparser "go/parser"
	"go/token"
	"go/types"
	"testing"

	"github.com/DataDog/orchestrion/internal/injector/aspect/advice"
	"github.com/DataDog/orchestrion/internal/injector/aspect/advice/code"
	"github.com/DataDog/orchestrion/internal/injector/aspect/context"
	"github.com/DataDog/orchestrion/internal/injector/typed"
	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
	"github.com/dave/dst/dstutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Test context -------------------------------------------------------------
//
// adviceCtx is a minimal AdviceContext backed by real type information: the
// test source is parsed, type-checked, and decorated once, and ResolveType
// replicates the production lookup. It exists so tests can apply advice to
// call shapes the public join-point matchers cannot select, and can assert on
// real *types.Tuple values.

type adviceCtx struct {
	chain  *context.NodeChain
	cursor *dstutil.Cursor
	file   *dst.File

	dec     *decorator.Decorator
	info    *types.Info
	nodeMap map[dst.Node]goast.Node
	imp     types.Importer

	imports   map[string]string
	links     []string
	minGoLang context.GoLangVersion
}

func (c *adviceCtx) Chain() *context.NodeChain   { return c.chain }
func (c *adviceCtx) Node() dst.Node              { return c.chain.Node() }
func (*adviceCtx) Parent() context.AspectContext { return nil }
func (*adviceCtx) Config(string) (string, bool)  { return "", false }
func (c *adviceCtx) File() *dst.File             { return c.file }
func (*adviceCtx) ImportPath() string            { return "test" }
func (*adviceCtx) Package() string               { return "test" }
func (*adviceCtx) TestMain() bool                { return false }
func (*adviceCtx) Release()                      {}
func (c *adviceCtx) Importer() types.Importer    { return c.imp }

func (c *adviceCtx) ParseSource(src []byte) (*dst.File, error) {
	return c.dec.Parse(src)
}

func (c *adviceCtx) AddImport(path string, alias string) bool {
	if c.imports == nil {
		c.imports = make(map[string]string)
	}
	if _, found := c.imports[path]; !found {
		c.imports[path] = alias
	}
	return true
}

func (c *adviceCtx) AddLink(path string) bool {
	c.links = append(c.links, path)
	return true
}

func (c *adviceCtx) EnsureMinGoLang(lang context.GoLangVersion) {
	c.minGoLang.SetAtLeast(lang)
}

func (c *adviceCtx) ReplaceNode(node dst.Node) {
	if c.cursor != nil {
		c.cursor.Replace(node)
	}
}

// Child is not used by the advice under test (only wrap-expression calls it);
// report loudly if that assumption ever breaks.
func (*adviceCtx) Child(dst.Node, string, int) context.AdviceContext {
	panic("adviceCtx.Child is not implemented")
}

// ResolveType replicates the production context lookup: the dst node is mapped
// back to its ast counterpart, and the type comes from types.Info.
func (c *adviceCtx) ResolveType(expr dst.Expr) types.Type {
	astNode, ok := c.nodeMap[expr]
	if !ok {
		return nil
	}
	astExpr, ok := astNode.(goast.Expr)
	if !ok {
		return nil
	}
	if tv, ok := c.info.Types[astExpr]; ok {
		return tv.Type
	}
	if ident, ok := astExpr.(*goast.Ident); ok {
		if obj, ok := c.info.Uses[ident]; ok {
			return obj.Type()
		}
	}
	if sel, ok := astExpr.(*goast.SelectorExpr); ok {
		if obj, ok := c.info.Uses[sel.Sel]; ok {
			return obj.Type()
		}
	}
	return nil
}

// --- Test environment ----------------------------------------------------------

type applyResult struct {
	modified bool
	err      error
	ctx      *adviceCtx
}

type adviceEnv struct {
	file  *dst.File
	dec   *decorator.Decorator
	info  *types.Info
	imp   types.Importer
	calls []*dst.CallExpr
}

func newAdviceEnv(t *testing.T, src string) *adviceEnv {
	t.Helper()

	fset := token.NewFileSet()
	astFile, err := goparser.ParseFile(fset, "input.go", src, goparser.ParseComments)
	require.NoError(t, err, "failed to parse test source")

	info := &types.Info{
		Types: make(map[goast.Expr]types.TypeAndValue),
		Uses:  make(map[*goast.Ident]types.Object),
	}
	imp := importer.Default()
	conf := types.Config{Importer: imp}
	_, err = conf.Check("test", fset, []*goast.File{astFile}, info)
	require.NoError(t, err, "failed to type-check test source")

	dec := decorator.NewDecorator(fset)
	file, err := dec.DecorateFile(astFile)
	require.NoError(t, err, "failed to decorate test source")

	env := &adviceEnv{file: file, dec: dec, info: info, imp: imp}
	dst.Inspect(file, func(node dst.Node) bool {
		if call, ok := node.(*dst.CallExpr); ok {
			env.calls = append(env.calls, call)
		}
		return true
	})
	return env
}

// calleeName renders the callee of a call the way advice error messages name
// calls: bare identifiers by name, selectors as X.Sel.
func calleeName(fun dst.Expr) string {
	switch fun := fun.(type) {
	case *dst.Ident:
		return fun.Name
	case *dst.SelectorExpr:
		return fmt.Sprintf("%s.%s", calleeName(fun.X), fun.Sel.Name)
	default:
		return ""
	}
}

func matchByCallee(name string) func(*dst.CallExpr) bool {
	return func(call *dst.CallExpr) bool { return calleeName(call.Fun) == name }
}

// apply runs the advice against every call matching match, in source order,
// building the node chain exactly the way the injector's traversal does.
func (e *adviceEnv) apply(t *testing.T, adv advice.Advice, match func(*dst.CallExpr) bool) []applyResult {
	t.Helper()

	dec := e.dec
	var results []applyResult
	var chain *context.NodeChain

	dstutil.Apply(e.file, func(csor *dstutil.Cursor) bool {
		chain = chain.Child(csor)
		return true
	}, func(csor *dstutil.Cursor) bool {
		defer func() {
			old := chain
			chain = chain.Parent()
			old.Release()
		}()

		call, ok := csor.Node().(*dst.CallExpr)
		if !ok || !match(call) {
			return true
		}

		ctx := &adviceCtx{
			chain:   chain,
			cursor:  csor,
			file:    e.file,
			dec:     dec,
			info:    e.info,
			nodeMap: dec.Ast.Nodes,
			imp:     e.imp,
		}
		modified, err := adv.Apply(ctx)
		results = append(results, applyResult{modified: modified, err: err, ctx: ctx})
		return err == nil
	})

	return results
}

func (e *adviceEnv) callByName(t *testing.T, name string) *dst.CallExpr {
	t.Helper()
	for _, call := range e.calls {
		if calleeName(call.Fun) == name {
			return call
		}
	}
	t.Fatalf("no call to %q in test source", name)
	return nil
}

// countNode counts the occurrences of target in root by node identity. It is
// the AST-level oracle for "the original expression is evaluated exactly once".
func countNode(root dst.Node, target dst.Node) int {
	count := 0
	dstutil.Apply(root, func(csor *dstutil.Cursor) bool {
		if csor.Node() == target {
			count++
		}
		return true
	}, nil)
	return count
}

// assertSpreadHygiene implements the strengthened AST invariant from the #949
// review: the rewritten spread argument contains no function literal,
// declaration, or assignment statement, and every identifier the advice
// synthesizes carries a non-empty import path. Identifiers inside the moved
// original expression are excluded, as are the type identifiers rendered from
// TypeName.AsNode() when the configured type is built-in (allowedTypeName,
// empty when the configured type is qualified).
func assertSpreadHygiene(t *testing.T, arg dst.Expr, original dst.Expr, allowedTypeName string) {
	t.Helper()

	var bad dst.Node
	dstutil.Apply(arg, nil, func(csor *dstutil.Cursor) bool {
		switch csor.Node().(type) {
		case *dst.FuncLit, *dst.DeclStmt, *dst.AssignStmt:
			bad = csor.Node()
		}
		return bad == nil
	})
	require.Nil(t, bad, "the spread rewrite must not introduce a %T", bad)

	dstutil.Apply(arg, func(csor *dstutil.Cursor) bool {
		if csor.Node() == dst.Node(original) {
			return false // the moved original expression is not synthesized
		}
		if id, ok := csor.Node().(*dst.Ident); ok && id.Path == "" {
			allowed := allowedTypeName != "" && id.Name == allowedTypeName
			if allowed {
				// Only type positions rendered by TypeName.AsNode() may be bare.
				switch parent := csor.Parent().(type) {
				case *dst.ArrayType:
					allowed = parent.Elt == dst.Expr(id)
				case *dst.CompositeLit:
					allowed = parent.Type == dst.Expr(id)
				default:
					allowed = false
				}
			}
			require.True(t, allowed,
				"the spread rewrite must not synthesize the unqualified identifier %q; only "+
					"import-path-qualified identifiers and the configured type name are allowed", id.Name)
		}
		return true
	}, nil)
}

// --- Tests ---------------------------------------------------------------------

func TestAppendArgs(t *testing.T) {
	t.Run("AddedImports", func(t *testing.T) {
		type testCase struct {
			argType         typed.TypeName
			args            []*code.Template
			expectedImports []string
		}

		testCases := map[string]testCase{
			"imports-none": {
				argType: typed.Any,
				args:    []*code.Template{code.MustTemplate("true", nil, context.GoLangVersion{})},
				// slices is used when the call spreads a slice into the variadic
				// parameter (V07); the metadata cannot know the call form, so it is
				// reported for any non-empty advice.
				expectedImports: []string{"slices"},
			},
			"imports-from-arg-type": {
				argType:         typed.MustTypeName("*net/http.Request"),
				args:            []*code.Template{code.MustTemplate("true", nil, context.GoLangVersion{})},
				expectedImports: []string{"slices", "net/http"},
			},
			"imports-from-templates": {
				argType: typed.Any,
				args: []*code.Template{
					code.MustTemplate("imp.Value", map[string]string{"imp": "github.com/namespace/foo"}, context.GoLangVersion{}),
					code.MustTemplate("imp.Value", map[string]string{"imp": "github.com/namespace/bar"}, context.GoLangVersion{}),
				},
				expectedImports: []string{
					"slices",
					"github.com/namespace/foo",
					"github.com/namespace/bar",
				},
			},
			// Z05: a zero-template advice reports no imports at all, even when the
			// configured type comes from another package: the advice is a no-op.
			"zero-templates-no-imports": {
				argType:         typed.MustTypeName("*net/http.Request"),
				args:            nil,
				expectedImports: nil,
			},
		}

		for name, tc := range testCases {
			t.Run(name, func(t *testing.T) {
				imports := advice.AppendArgs(tc.argType, tc.args...).AddedImports()
				for _, imp := range tc.expectedImports {
					assert.Contains(t, imports, imp)
				}
				require.Len(t, imports, len(tc.expectedImports), "expected %d imports, got %d: %v", len(tc.expectedImports), len(imports), imports)
				// V07: slices is reported exactly once, for any non-empty advice.
				if len(tc.args) > 0 {
					assert.Equal(t, 1, countStrings(imports, "slices"), "slices must be reported exactly once: %v", imports)
				}
			})
		}
	})

	// Z01-Z04, V05: a zero-template advice is a true no-op for every call form:
	// it never rewrites the call, never requests an import, and never requests a
	// language version. A sole tuple is accepted because nothing is appended.
	t.Run("ZeroTemplates", func(t *testing.T) {
		src := `package test

func sink(vals ...any) {}

func two() (any, any) { return 1, 2 }

func spread(xs []any) { sink(xs...) }

func direct(x any) { sink(x) }

func tuple() { sink(two()) }
`
		adv := advice.AppendArgs(typed.Any)

		t.Run("Spread", func(t *testing.T) {
			env := newAdviceEnv(t, src)
			spreadCall := env.apply(t, adv, func(c *dst.CallExpr) bool {
				return calleeName(c.Fun) == "sink" && c.Ellipsis
			})
			require.Len(t, spreadCall, 1)
			call := env.callByName(t, "sink")
			orig := call.Args[len(call.Args)-1]
			assert.False(t, spreadCall[0].modified, "a zero-template advice must not modify a spread call")
			require.NoError(t, spreadCall[0].err)
			assert.Empty(t, spreadCall[0].ctx.imports, "a zero-template advice must not request imports")
			assert.True(t, spreadCall[0].ctx.minGoLang.IsAny(), "a zero-template advice must not request a language version")
			assert.Same(t, orig, call.Args[len(call.Args)-1], "the spread argument must be left in place")
		})

		t.Run("Direct", func(t *testing.T) {
			env := newAdviceEnv(t, src)
			direct := env.apply(t, adv, func(c *dst.CallExpr) bool {
				return calleeName(c.Fun) == "sink" && len(c.Args) == 1 && !c.Ellipsis && !isCallExpr(c.Args[0])
			})
			require.Len(t, direct, 1)
			assert.False(t, direct[0].modified, "a zero-template advice must not modify a direct call")
			require.NoError(t, direct[0].err)
			assert.Empty(t, direct[0].ctx.imports)
			assert.True(t, direct[0].ctx.minGoLang.IsAny())
		})

		t.Run("Tuple", func(t *testing.T) {
			env := newAdviceEnv(t, src)
			tuple := env.apply(t, adv, func(c *dst.CallExpr) bool {
				if len(c.Args) != 1 {
					return false
				}
				arg, ok := c.Args[0].(*dst.CallExpr)
				return ok && calleeName(arg.Fun) == "two"
			})
			require.Len(t, tuple, 1)
			assert.False(t, tuple[0].modified, "a zero-template advice must not modify a tuple call")
			assert.NoError(t, tuple[0].err, "a zero-template advice must accept a sole tuple")
		})
	})

	// V01, V06: the spread rewrite requires exactly go1.18 for the explicit
	// instantiation; it must never request go1.22, which would change loop
	// variable semantics for go1.21 modules.
	t.Run("SpreadMinimumLanguage", func(t *testing.T) {
		src := `package test

func sink(vals ...any) {}

func f(xs []any) { sink(xs...) }
`
		env := newAdviceEnv(t, src)
		adv := advice.AppendArgs(typed.Any, code.MustTemplate(`"woven"`, nil, context.GoLangVersion{}))
		res := env.apply(t, adv, matchByCallee("sink"))
		require.Len(t, res, 1)
		require.NoError(t, res[0].err)
		assert.True(t, res[0].modified)
		assert.Equal(t, "go1.18", res[0].ctx.minGoLang.String(), "the spread rewrite must request exactly go1.18")
	})

	// C05, C06, C05b: a non-spread call whose sole argument is a multi-value
	// expression cannot take extra arguments; the advice must reject it with an
	// error that names the call, the restriction, and the value count. The
	// detection uses the resolved type, not the surface syntax, so one-result
	// calls in any shape stay allowed (see OneResultAllowed).
	t.Run("SoleTupleRejected", func(t *testing.T) {
		testCases := []struct {
			name   string
			src    string
			callee string
			count  int
			called string
		}{
			{
				name: "two-results",
				src: `package test
func sink(vals ...any) {}
func two() (any, any) { return 1, 2 }
func f() { sink(two()) }
`,
				callee: "sink", count: 2, called: "sink",
			},
			{
				name: "three-results",
				src: `package test
func sink(vals ...any) {}
func three() (any, any, any) { return 1, 2, 3 }
func f() { sink(three()) }
`,
				callee: "sink", count: 3, called: "sink",
			},
			{
				name: "named-results",
				src: `package test
func sink(vals ...any) {}
func named() (a, b any) { return }
func f() { sink(named()) }
`,
				callee: "sink", count: 2, called: "sink",
			},
			{
				name: "func-value",
				src: `package test
func sink(vals ...any) {}
var fn = func() (any, any) { return 1, 2 }
func f() { sink(fn()) }
`,
				callee: "sink", count: 2, called: "sink",
			},
			{
				name: "method-call",
				src: `package test
func sink(vals ...any) {}
type T struct{}
func (T) Ret2() (any, any) { return 1, 2 }
func f(t T) { sink(t.Ret2()) }
`,
				callee: "sink", count: 2, called: "sink",
			},
			{
				// C05b: f(two()) is valid Go when f has a fixed parameter and a
				// variadic tail, so the advice must still reject it.
				name: "fixed-split",
				src: `package test
func fixed(a any, rest ...any) {}
func two() (any, any) { return 1, 2 }
func f() { fixed(two()) }
`,
				callee: "fixed", count: 2, called: "fixed",
			},
			{
				name: "forwarding",
				src: `package test
func sink(vals ...any) {}
func two() (any, any) { return 1, 2 }
func fwd() (any, any) { return two() }
func f() { sink(fwd()) }
`,
				callee: "sink", count: 2, called: "sink",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				env := newAdviceEnv(t, tc.src)
				call := env.callByName(t, tc.callee)
				origArg := call.Args[0]
				adv := advice.AppendArgs(typed.Any, code.MustTemplate(`"woven"`, nil, context.GoLangVersion{}))
				res := env.apply(t, adv, matchByCallee(tc.callee))
				require.Len(t, res, 1, "expected exactly one matching call")
				require.Error(t, res[0].err, "the sole multi-value call must be rejected")
				assert.False(t, res[0].modified, "a rejected call must not be rewritten")
				msg := res[0].err.Error()
				assert.Contains(t, msg, tc.called, "the error must name the call: %s", msg)
				assert.Contains(t, msg, fmt.Sprintf("%d values", tc.count), "the error must state the value count: %s", msg)
				assert.Contains(t, msg, "append-arguments", "the error must state the restriction: %s", msg)
				// The invalid argument list must not be extended.
				assert.Same(t, origArg, call.Args[0], "a rejected call must keep its argument")
				assert.Len(t, call.Args, 1)
			})
		}
	})

	// C02, C02b: a sole argument that only looks like a multi-value call has a
	// one-value type and must keep receiving the injected values.
	t.Run("OneResultAllowed", func(t *testing.T) {
		testCases := []struct {
			name string
			src  string
		}{
			{
				name: "call",
				src: `package test
func sink(vals ...any) {}
func id(v any) any { return v }
func f(x any) { sink(id(x)) }
`,
			},
			{
				name: "conversion",
				src: `package test
func sink(vals ...any) {}
func f(x int) { sink(any(x)) }
`,
			},
			{
				name: "parenthesized",
				src: `package test
func sink(vals ...any) {}
func g() any { return 1 }
func f() { sink((g())) }
`,
			},
			{
				name: "generic-explicit",
				src: `package test
func sink(vals ...any) {}
func id[T any](v T) T { return v }
func f(x int) { sink(id[int](x)) }
`,
			},
			{
				name: "method-value",
				src: `package test
func sink(vals ...any) {}
type T struct{}
func (T) M() any { return 1 }
func f(t T) { mv := t.M; sink(mv()) }
`,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				env := newAdviceEnv(t, tc.src)
				call := env.callByName(t, "sink")
				adv := advice.AppendArgs(typed.Any, code.MustTemplate(`"woven"`, nil, context.GoLangVersion{}))
				res := env.apply(t, adv, matchByCallee("sink"))
				require.Len(t, res, 1)
				require.NoError(t, res[0].err, "a one-result sole argument must not be rejected")
				assert.True(t, res[0].modified)
				assert.Len(t, call.Args, 2, "exactly one value must be appended")
				assert.Equal(t, 1, countNode(call, call.Args[0]), "the original argument must survive exactly once")
			})
		}
	})

	// C11 (tuple part): with two non-empty aspects, the first one rejects the
	// tuple call before an invalid rewrite can happen.
	t.Run("TupleWithTwoAspects", func(t *testing.T) {
		src := `package test

func sink(vals ...any) {}

func two() (any, any) { return 1, 2 }

func f() { sink(two()) }
`
		env := newAdviceEnv(t, src)
		first := advice.AppendArgs(typed.Any, code.MustTemplate(`"first"`, nil, context.GoLangVersion{}))
		second := advice.AppendArgs(typed.Any, code.MustTemplate(`"second"`, nil, context.GoLangVersion{}))

		res := env.apply(t, first, matchByCallee("sink"))
		require.Len(t, res, 1)
		require.Error(t, res[0].err, "the first non-empty advice must reject the tuple call")

		// The second aspect never gets to run on this call through the real
		// injector (the error aborts the file); mirror that by asserting the call
		// is untouched after the rejection.
		call := env.callByName(t, "sink")
		assert.Len(t, call.Args, 1)
		secondRes := env.apply(t, second, matchByCallee("sink"))
		require.Len(t, secondRes, 1)
		require.Error(t, secondRes[0].err, "the second aspect must also reject the unchanged tuple call")
	})

	// C12: an empty aspect composes as a no-op in both orders.
	t.Run("EmptyAspectComposition", func(t *testing.T) {
		src := `package test

func sink(vals ...any) {}

func f(xs []any) { sink(xs...) }
`
		empty := advice.AppendArgs(typed.Any)
		nonEmpty := advice.AppendArgs(typed.Any, code.MustTemplate(`"woven"`, nil, context.GoLangVersion{}))

		t.Run("EmptyFirst", func(t *testing.T) {
			env := newAdviceEnv(t, src)
			call := env.callByName(t, "sink")
			orig := call.Args[len(call.Args)-1]

			res := env.apply(t, empty, matchByCallee("sink"))
			require.Len(t, res, 1)
			assert.False(t, res[0].modified, "the empty aspect must be a no-op")
			assert.Same(t, orig, call.Args[len(call.Args)-1], "the empty aspect must not touch the call")

			res = env.apply(t, nonEmpty, matchByCallee("sink"))
			require.Len(t, res, 1)
			require.NoError(t, res[0].err)
			assert.True(t, res[0].modified)
			assert.Equal(t, 1, countNode(call, orig), "the original spread argument must survive exactly once")
		})

		t.Run("NonEmptyFirst", func(t *testing.T) {
			env := newAdviceEnv(t, src)
			call := env.callByName(t, "sink")

			res := env.apply(t, nonEmpty, matchByCallee("sink"))
			require.Len(t, res, 1)
			require.NoError(t, res[0].err)
			assert.True(t, res[0].modified)
			woven := call.Args[len(call.Args)-1]

			res = env.apply(t, empty, matchByCallee("sink"))
			require.Len(t, res, 1)
			assert.False(t, res[0].modified, "the empty aspect must be a no-op after another aspect")
			assert.Same(t, woven, call.Args[len(call.Args)-1], "the empty aspect must not touch the woven call")
		})
	})

	// C13: after one aspect injects a call-expression value, the second aspect
	// sees a sole argument that has no type information (it is synthesized, so
	// it maps to no ast node and ResolveType returns nil). The expected result is
	// to fail open: append, do not reject.
	t.Run("TupleCheckSyntheticArg", func(t *testing.T) {
		src := `package test

func sink(vals ...any) {}

func id(v any) any { return v }

func f() { sink() }
`
		env := newAdviceEnv(t, src)
		call := env.callByName(t, "sink")
		first := advice.AppendArgs(typed.Any, code.MustTemplate(`id(1)`, nil, context.GoLangVersion{}))
		second := advice.AppendArgs(typed.Any, code.MustTemplate(`id(2)`, nil, context.GoLangVersion{}))

		res := env.apply(t, first, matchByCallee("sink"))
		require.Len(t, res, 1)
		require.NoError(t, res[0].err)
		require.Len(t, call.Args, 1)

		// The synthesized argument has no type information for the tuple check.
		assert.Nil(t, res[0].ctx.ResolveType(call.Args[0]),
			"a synthesized argument maps to no ast node, so ResolveType is nil")

		res = env.apply(t, second, matchByCallee("sink"))
		require.Len(t, res, 1)
		require.NoError(t, res[0].err, "a nil ResolveType must fail open, not reject")
		assert.True(t, res[0].modified)
		assert.Len(t, call.Args, 2, "the second aspect must append its value")
	})

	// E04 (unmatched shapes): a function-value call and a call through a
	// receiver factory cannot be selected by the public matchers; applied
	// directly, the spread rewrite must still evaluate the original expressions
	// exactly once. Counting node occurrences is the AST-level oracle for this.
	t.Run("EvaluationCallShapes", func(t *testing.T) {
		src := `package test

func sink(vals ...any) {}

func source() []any { return nil }

type fnT func(vals ...any)

var fn fnT

var factory = func() fnT { return fn }

			func f(xs []any) {
				fn(source()...)
				factory()(source()...)
			}
`
		env := newAdviceEnv(t, src)
		adv := advice.AppendArgs(typed.Any, code.MustTemplate(`"woven"`, nil, context.GoLangVersion{}))

		fnCall := env.calls[0]
		require.Equal(t, "fn", calleeName(fnCall.Fun))
		fnOrig := fnCall.Args[0]

		var factoryCall *dst.CallExpr
		for _, c := range env.calls {
			if _, ok := c.Fun.(*dst.CallExpr); ok {
				factoryCall = c
			}
		}
		require.NotNil(t, factoryCall, "the factory call has a call-expression callee")
		factoryOrig := factoryCall.Args[0]

		res := env.apply(t, adv, func(c *dst.CallExpr) bool {
			return len(c.Args) == 1 && c.Ellipsis
		})
		require.Len(t, res, 2)
		for _, r := range res {
			require.NoError(t, r.err)
			assert.True(t, r.modified)
		}

		assert.Equal(t, 1, countNode(dst.Node(fnCall), fnOrig), "the spread expression must appear exactly once")
		assert.Equal(t, 1, countNode(dst.Node(factoryCall), factoryOrig), "the spread expression must appear exactly once")
		assert.True(t, fnCall.Ellipsis)
		assert.True(t, factoryCall.Ellipsis)
	})

	// C09: function-value calls and calls through factory expressions transform
	// like any other call once selected.
	t.Run("FunctionValueCallShape", func(t *testing.T) {
		src := `package test

func sink(vals ...any) {}

func source() []any { return nil }

type fnT func(vals ...any)

var fn fnT

var factory = func() fnT { return fn }

			func f(xs []any) {
				fn(source()...)
				factory()(source()...)
			}
`
		env := newAdviceEnv(t, src)
		adv := advice.AppendArgs(typed.Any, code.MustTemplate(`"woven"`, nil, context.GoLangVersion{}))
		res := env.apply(t, adv, func(c *dst.CallExpr) bool {
			return len(c.Args) == 1 && c.Ellipsis
		})
		require.Len(t, res, 2)
		for _, r := range res {
			require.NoError(t, r.err)
			assert.True(t, r.modified)
		}
	})

	// Strengthened AST invariant from the #949 review: the spread rewrite
	// introduces no new local names, no function literal, no declaration or
	// assignment, and no unqualified predeclared identifier. Every synthesized
	// identifier carries a non-empty import path, except the identifiers that
	// come from TypeName.AsNode() when the configured type is built-in.
	t.Run("SpreadIntroducesNoBindings", func(t *testing.T) {
		src := `package test

func sink(vals ...any) {}

func source() []any { return nil }

func f() { sink(source()...) }
`
		t.Run("QualifiedType", func(t *testing.T) {
			env := newAdviceEnv(t, src)
			call := env.callByName(t, "sink")
			orig := call.Args[len(call.Args)-1]
			adv := advice.AppendArgs(
				typed.MustTypeName("net/http.Request"),
				code.MustTemplate("imp.Value", map[string]string{"imp": "github.com/namespace/foo"}, context.GoLangVersion{}),
			)
			res := env.apply(t, adv, matchByCallee("sink"))
			require.Len(t, res, 1)
			require.NoError(t, res[0].err)
			assertSpreadHygiene(t, call.Args[len(call.Args)-1], orig, "")
		})

		t.Run("BuiltinType", func(t *testing.T) {
			env := newAdviceEnv(t, src)
			call := env.callByName(t, "sink")
			orig := call.Args[len(call.Args)-1]
			adv := advice.AppendArgs(
				typed.Any,
				code.MustTemplate("imp.Value", map[string]string{"imp": "github.com/namespace/foo"}, context.GoLangVersion{}),
			)
			res := env.apply(t, adv, matchByCallee("sink"))
			require.Len(t, res, 1)
			require.NoError(t, res[0].err)
			assertSpreadHygiene(t, call.Args[len(call.Args)-1], orig, "any")
		})
	})
}

func countStrings(values []string, target string) int {
	count := 0
	for _, v := range values {
		if v == target {
			count++
		}
	}
	return count
}

func isCallExpr(expr dst.Expr) bool {
	_, ok := expr.(*dst.CallExpr)
	return ok
}
