// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package advice_test

import (
	"testing"

	"github.com/DataDog/orchestrion/internal/injector/aspect/advice"
	"github.com/DataDog/orchestrion/internal/injector/aspect/context"
	"github.com/DataDog/orchestrion/internal/injector/aspect/context/contexttest"
	"github.com/dave/dst"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	redirectImportPath = "example.com/advicetest"

	// redirectDecls declares the generic function family used by the
	// replace-function tests, along with their replacement counterparts.
	redirectDecls = `
func F[A any](a A) A                          { return a }
func FR[A any](a A) A                           { return a }
func G[A, B any](a A, b B) B                    { var z B; return z }
func GR[A, B any](a A, b B) B                   { var z B; return z }
func H[A, B, C, D any](a A, b B, c C, d D) D    { var z D; return z }
func HR[A, B, C, D any](a A, b B, c C, d D) D   { var z D; return z }
func V3[A, B, C any](a A, b B, c C) C           { var z C; return z }
func V3R[A, B, C any](a A, b B, c C) C          { var z C; return z }
func Any1[A any](a A) A                         { return a }
func Num1[A ~int](a A) A                        { return a }
func Ptr2[A, B any](a A) *B                     { return nil }
func Plain(a int) int                           { return a }
func Zero() int                                { return 0 }

var handlers = []func() int{func() int { return 0 }}
`
)

func buildRedirectSrc(src string) string {
	return "package test\n" + redirectDecls + src
}

// applyReplace applies the replace-function advice to the single call whose
// callee renders as callee, and returns the modified call.
func applyReplace(t *testing.T, fx *contexttest.Fixture, callee string, target string) *dst.CallExpr {
	t.Helper()

	call := fx.FindCall(t, callee)

	path, name := splitTarget(t, target)
	redirect := advice.ReplaceFunction(path, name)

	var applied bool
	err := fx.WalkAdvice(func(ctx context.AdviceContext) error {
		if ctx.Node() != dst.Node(call) {
			return nil
		}
		changed, err := redirect.Apply(ctx)
		applied = changed
		return err
	})
	require.NoError(t, err)
	require.True(t, applied, "the advice must have applied to %q", callee)

	return call
}

func splitTarget(t *testing.T, symbol string) (path string, name string) {
	t.Helper()

	// Mirrors the replace-function YAML unmarshaler's split.
	for i := len(symbol) - 1; i >= 0; i-- {
		if symbol[i] == '.' {
			return symbol[:i], symbol[i+1:]
		}
	}
	return "", symbol
}

// TestReplaceFunctionTypeArgs covers replace-function on confirmed generic
// instantiations: only the function operand is replaced, and the explicit
// type arguments are retained in their original order (matrix A11-A17). Valid
// outputs type-check; incompatible replacements produce the compiler error the
// retained type list causes, never silently adapted arguments.
func TestReplaceFunctionTypeArgs(t *testing.T) {
	tests := []struct {
		name string
		// src is appended to the standard declarations.
		src string
		// extra declares synthetic external packages (path -> source).
		extra map[string]string
		// imports is the import block of the source, if any.
		imports string
		// callee is the rendered callee the advice is applied to.
		callee string
		// target is the replacement symbol.
		target string
		// wantCallee is the expected rendered callee after the advice ran.
		wantCallee string
		// wantErr, when not empty, is a substring of the type-check error the
		// rendered output must produce. Empty means the output must compile.
		wantErr string
	}{
		{
			name:       "arity one",
			src:        "\nfunc Use() int { return F[int](1) }\n",
			callee:     "F[int]",
			target:     "FR",
			wantCallee: "FR[int]",
		},
		{
			name:       "arity two",
			src:        "\nfunc Use() string { return G[int, string](1, \"x\") }\n",
			callee:     "G[int, string]",
			target:     "GR",
			wantCallee: "GR[int, string]",
		},
		{
			name:       "arity four",
			src:        "\nfunc Use() float64 { return H[int, string, bool, float64](1, \"x\", true, 2.0) }\n",
			callee:     "H[int, string, bool, float64]",
			target:     "HR",
			wantCallee: "HR[int, string, bool, float64]",
		},
		{
			name:       "partial instantiation",
			src:        "\nfunc Use() string { return G[int](1, \"x\") }\n",
			callee:     "G[int]",
			target:     "GR",
			wantCallee: "GR[int]",
		},
		{
			name:       "inferable trailing type parameters",
			src:        "\nfunc Use() bool { return V3[int, string](1, \"x\", true) }\n",
			callee:     "V3[int, string]",
			target:     "V3R",
			wantCallee: "V3R[int, string]",
		},
		{
			name:       "replacement with fewer type parameters keeps the full list",
			src:        "\nfunc Use() float64 { return H[int, string, bool, float64](1, \"x\", true, 2.0) }\n",
			callee:     "H[int, string, bool, float64]",
			target:     "GR",
			wantCallee: "GR[int, string, bool, float64]",
			wantErr:    "got 4 type arguments",
		},
		{
			name:       "uninferable trailing type parameter",
			src:        "\nfunc Use() *int { return F[*int](nil) }\n",
			callee:     "F[*int]",
			target:     "Ptr2",
			wantCallee: "Ptr2[*int]",
			wantErr:    "infer",
		},
		{
			name:       "constraint violation",
			src:        "\nfunc Use() []int { return Any1[[]int](nil) }\n",
			callee:     "Any1[[]int]",
			target:     "Num1",
			wantCallee: "Num1[[]int]",
			wantErr:    "does not satisfy",
		},
		{
			name:       "non-generic replacement keeps the type list",
			src:        "\nfunc Use() int { return F[int](1) }\n",
			callee:     "F[int]",
			target:     "Plain",
			wantCallee: "Plain[int]",
			wantErr:    "Plain",
		},
		{
			name:       "inferred generic call to a non-generic replacement",
			src:        "\nfunc Use() int { return F(1) }\n",
			callee:     "F",
			target:     "Plain",
			wantCallee: "Plain",
		},
		{
			name:       "non-generic call to a generic replacement",
			src:        "\nfunc Use() int { return Plain(1) }\n",
			callee:     "Plain",
			target:     "F",
			wantCallee: "F",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fx := contexttest.New(t, redirectImportPath, buildRedirectSrc(tc.src), tc.extra)

			call := applyReplace(t, fx, tc.callee, tc.target)

			assert.Equal(t, tc.wantCallee, contexttest.RenderExpr(call.Fun), "unexpected rewritten callee")

			err := contexttest.CheckSource(t, redirectImportPath, fx.Source(t), tc.extra)
			if tc.wantErr == "" {
				require.NoError(t, err, "the rewritten source must type-check")
				return
			}
			require.Error(t, err, "the rewritten source must fail type-checking")
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}

// TestReplaceFunctionIndexedCallee guards whole-callee replacement for indexed
// callees that are not confirmed generic instantiations: ordinary value
// indexes, callees without type information, and non-function index-list
// operands such as generic type conversions (matrix A20-A22). The saved
// prototype unwrapped all of these; it must not regress that way.
func TestReplaceFunctionIndexedCallee(t *testing.T) {
	t.Run("value index", func(t *testing.T) {
		fx := contexttest.New(t, redirectImportPath, buildRedirectSrc("\nfunc Use() int { return handlers[0]() }\n"), nil)

		call := applyReplace(t, fx, "handlers[0]", "Zero")

		_, isIdent := call.Fun.(*dst.Ident)
		assert.True(t, isIdent, "the whole callee must be replaced, not only its operand")
		assert.Equal(t, "Zero", contexttest.RenderExpr(call.Fun))
		require.NoError(t, contexttest.CheckSource(t, redirectImportPath, fx.Source(t), nil))
	})

	t.Run("unresolved index", func(t *testing.T) {
		fx := contexttest.New(t, redirectImportPath, buildRedirectSrc("\nfunc Use() int { return F[int](1) }\n"), nil)
		call := fx.FindCall(t, "F[int]")
		operand := call.Fun.(*dst.IndexExpr).X
		require.NotNil(t, fx.Decorator.Ast.Nodes[operand])
		delete(fx.Decorator.Ast.Nodes, operand)

		call = applyReplace(t, fx, "F[int]", "Plain")

		_, isIdent := call.Fun.(*dst.Ident)
		assert.True(t, isIdent, "an indexed callee without type information must be replaced whole")
		assert.Equal(t, "Plain", contexttest.RenderExpr(call.Fun))
	})

	t.Run("non-function index-list operand", func(t *testing.T) {
		extra := map[string]string{
			"example.com/genlib": `package genlib

type Pair[A, B any] func(A) B
`,
		}
		src := "package test\nimport (\n\"example.com/genlib\"\n)" + redirectDecls + `
func Use(f func(int) string) genlib.Pair[int, string] {
	return genlib.Pair[int, string](f)
}
`
		fx := contexttest.New(t, redirectImportPath, src, extra)

		call := applyReplace(t, fx, "example.com/genlib.Pair[int, string]", "F")

		_, isIdent := call.Fun.(*dst.Ident)
		assert.True(t, isIdent, "a type conversion callee must be replaced whole")
		assert.Equal(t, "F", contexttest.RenderExpr(call.Fun))
		require.NoError(t, contexttest.CheckSource(t, redirectImportPath, fx.Source(t), extra))
	})
}

// TestReplaceFunctionTypeArgsStructure asserts that a confirmed instantiation
// keeps its type arguments, value arguments, and spread syntax untouched
// (matrix replacement invariant, steps 1-3).
func TestReplaceFunctionTypeArgsStructure(t *testing.T) {
	t.Run("index list", func(t *testing.T) {
		src := "package test\n" + redirectDecls + `
func Use(xs []int) string {
	return G[int, string](len(xs), "x")
}
`
		fx := contexttest.New(t, redirectImportPath, src, nil)

		call := applyReplace(t, fx, "G[int, string]", "GR")

		redirected, ok := call.Fun.(*dst.IndexListExpr)
		require.True(t, ok, "the index list must be preserved")
		assert.Equal(t, "GR", contexttest.RenderExpr(redirected.X))
		require.Len(t, redirected.Indices, 2)
		assert.Equal(t, "int", contexttest.RenderExpr(redirected.Indices[0]))
		assert.Equal(t, "string", contexttest.RenderExpr(redirected.Indices[1]))
		require.Len(t, call.Args, 2)
		assert.Equal(t, "len(xs)", contexttest.RenderExpr(call.Args[0]))
		assert.False(t, call.Ellipsis, "a non-spread call must not gain an ellipsis")
	})

	t.Run("spread arguments", func(t *testing.T) {
		src := "package test\n" + redirectDecls + `
func Variadic2[A, B any](a A, rest ...B) B { var z B; return z }
func Variadic2R[A, B any](a A, rest ...B) B { var z B; return z }

func Use(xs []string) string {
	return Variadic2[int, string](1, xs...)
}
`
		fx := contexttest.New(t, redirectImportPath, src, nil)

		call := applyReplace(t, fx, "Variadic2[int, string]", "Variadic2R")

		redirected, ok := call.Fun.(*dst.IndexListExpr)
		require.True(t, ok, "the index list must be preserved")
		assert.Equal(t, "Variadic2R", contexttest.RenderExpr(redirected.X))
		require.NotNil(t, call.Ellipsis, "the spread syntax must be preserved")
		require.NoError(t, contexttest.CheckSource(t, redirectImportPath, fx.Source(t), nil))
	})
}
