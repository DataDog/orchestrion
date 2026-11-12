// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package code_test

import (
	"fmt"
	"testing"

	"github.com/DataDog/orchestrion/internal/injector/aspect/advice/code"
	"github.com/DataDog/orchestrion/internal/injector/aspect/context"
	"github.com/DataDog/orchestrion/internal/injector/aspect/context/contexttest"
	"github.com/dave/dst"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const nameTestImportPath = "example.com/codetest"

// compileForCall compiles the template as an expression in the context of the
// single call whose callee renders as callee, and returns the result.
func compileForCall(t *testing.T, fx *contexttest.Fixture, callee string, tmpl string) (dst.Expr, error) {
	t.Helper()

	call := fx.FindCall(t, callee)
	template := code.MustTemplate(tmpl, nil, context.GoLangVersion{})

	var (
		result dst.Expr
		err    error
		found  bool
	)
	require.NoError(t, fx.WalkAdvice(func(ctx context.AdviceContext) error {
		if ctx.Node() != dst.Node(call) {
			return nil
		}
		found = true
		result, err = template.CompileExpression(ctx)
		return nil
	}))
	require.True(t, found, "no context found for callee %q", callee)

	return result, err
}

// TestDotAstFunNameOnIndexedCallees covers {{ .AST.Fun.Name }} on index and
// index-list callees: it must return the leaf function name (matrix A09). It
// must not include the package or the type arguments.
func TestDotAstFunNameOnIndexedCallees(t *testing.T) {
	const src = `package test

import "slices"

func Use(xs []int) []int {
	_ = slices.Clone[[]int](xs)
	return slices.Clone[[]int, int](xs)
}
`
	fx := contexttest.New(t, nameTestImportPath, src, nil)

	for callee, want := range map[string]string{
		"slices.Clone[[]int]":      "Clone",
		"slices.Clone[[]int, int]": "Clone",
	} {
		t.Run(callee, func(t *testing.T) {
			// The name is quoted so an empty result still compiles as a string
			// literal and stays observable.
			result, err := compileForCall(t, fx, callee, `"{{ .AST.Fun.Name }}"`)
			require.NoError(t, err, "{{ .AST.Fun.Name }} must resolve on %q", callee)
			assert.Equal(t, fmt.Sprintf("%q", want), contexttest.RenderExpr(result))
		})
	}
}

// TestDotAstFunNameOnNonFunctionIndexes pins the .Name contract for indexed
// callees that are not generic instantiations (matrix T01, T02): the operand
// identifier name is returned, and the empty string when the operand is not an
// identifier or selector.
func TestDotAstFunNameOnNonFunctionIndexes(t *testing.T) {
	const src = `package test

import "slices"

type Wrap[T any] []T

var handlers = [][]func() int{{}}
var flat = []func() int{}

func mk() []func() int { return nil }

func Use() int {
	_ = slices.Clone[[]int](nil)
	_ = Wrap[int]([]int{})
	_ = handlers[0][1]()
	_ = mk()[0]()
	_ = (flat)[0]()
	return 0
}
`
	fx := contexttest.New(t, nameTestImportPath, src, nil)

	for callee, want := range map[string]string{
		"handlers[0][1]": "", // Nested index operand: not an identifier or selector.
		"mk()[0]":        "",
		"(flat)[0]":      "",
	} {
		t.Run(callee, func(t *testing.T) {
			// The name is quoted so an empty result still compiles as a string
			// literal and stays observable.
			result, err := compileForCall(t, fx, callee, `"{{ .AST.Fun.Name }}"`)
			require.NoError(t, err, "{{ .AST.Fun.Name }} must resolve on %q", callee)
			assert.Equal(t, fmt.Sprintf("%q", want), contexttest.RenderExpr(result))
		})
	}
}

// TestDotAstFunNameOnGenericTypeConversions covers .Name on a generic
// conversion callee (matrix T01): it returns the operand name, while the
// function-call join point itself never matches it.
func TestDotAstFunNameOnGenericTypeConversions(t *testing.T) {
	const src = `package test

type Wrap[T any] []T

func Use() Wrap[int] {
	return Wrap[int]([]int{})
}
`
	fx := contexttest.New(t, nameTestImportPath, src, nil)

	result, err := compileForCall(t, fx, "Wrap[int]", `"{{ .AST.Fun.Name }}"`)
	require.NoError(t, err, "{{ .AST.Fun.Name }} must resolve on a conversion callee")
	assert.Equal(t, `"Wrap"`, contexttest.RenderExpr(result))
}

// TestDotAstFunNameReconstructionLosesTypeArguments covers matrix A10:
// reconstructing a call through .Name alone succeeds at name lookup but loses
// the explicit type arguments, so the reconstructed call fails type inference
// when the arguments cannot be inferred from the values. Templates that must
// retain type arguments use {{ .AST.Fun }} instead.
func TestDotAstFunNameReconstructionLosesTypeArguments(t *testing.T) {
	const src = `package test

import "slices"

func Use() []int {
	return slices.Clone[[]int](nil)
}
`
	fx := contexttest.New(t, nameTestImportPath, src, nil)

	result, err := compileForCall(t, fx, "slices.Clone[[]int]", "slices.{{ .AST.Fun.Name }}(nil)")
	require.NoError(t, err, "the name reconstruction must compile")

	reconstructed := contexttest.RenderExpr(result)
	require.Equal(t, "slices.Clone(nil)", reconstructed, "the type arguments must be absent from the reconstruction")

	checkSrc := "package test\nimport \"slices\"\nfunc Use() []int { return " + reconstructed + " }"
	err = contexttest.CheckSource(t, nameTestImportPath, checkSrc, nil)
	require.Error(t, err, "slices.Clone(nil) must fail type inference")
	assert.Contains(t, err.Error(), "infer")
}
