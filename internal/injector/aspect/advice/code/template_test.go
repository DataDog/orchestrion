// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package code_test

import (
	"errors"
	"go/types"
	"testing"

	"github.com/datadog/orchestrion/internal/injector/aspect/advice/code"
	"github.com/datadog/orchestrion/internal/injector/aspect/context"
	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
	"github.com/stretchr/testify/require"
	"gotest.tools/v3/golden"
)

func TestTemplate(t *testing.T) {
	ctx := mockAdviceContext{}

	t.Run("ParseError", func(t *testing.T) {
		tmpl := code.MustTemplate(`this.IsNotValidGo("because it's missing a closing parenthesis"`, nil)
		stmt, err := tmpl.CompileBlock(ctx)
		require.Nil(t, stmt)
		require.Error(t, err)
		golden.Assert(t, err.Error(), "parse_error.txt")
	})
}

type mockAdviceContext struct{}

func (mockAdviceContext) ParseSource(src []byte) (*dst.File, error) {
	return decorator.Parse(src)
}

// The rest is not used by the tests as of now...
func (mockAdviceContext) Node() dst.Node {
	panic(errors.ErrUnsupported)
}
func (mockAdviceContext) Parent() context.AspectContext {
	panic(errors.ErrUnsupported)
}
func (mockAdviceContext) Config(string) (string, bool) {
	panic(errors.ErrUnsupported)
}
func (mockAdviceContext) File() *dst.File {
	panic(errors.ErrUnsupported)
}
func (mockAdviceContext) ImportPath() string {
	panic(errors.ErrUnsupported)
}
func (mockAdviceContext) Package() string {
	panic(errors.ErrUnsupported)
}
func (mockAdviceContext) Child(dst.Node, string, int) context.AdviceContext {
	panic(errors.ErrUnsupported)
}
func (mockAdviceContext) ReplaceNode(dst.Node) {
	panic(errors.ErrUnsupported)
}
func (mockAdviceContext) AddImport(path string, alias string) bool {
	panic(errors.ErrUnsupported)
}
func (mockAdviceContext) AddLink(string) bool {
	panic(errors.ErrUnsupported)
}
func (mockAdviceContext) TypeOf(expr dst.Expr) types.Type {
	panic(errors.ErrUnsupported)
}
