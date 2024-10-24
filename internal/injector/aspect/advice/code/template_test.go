// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package code_test

import (
	"testing"

	"github.com/DataDog/orchestrion/internal/injector/aspect/advice/code"
	"github.com/DataDog/orchestrion/internal/injector/aspect/context"
	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gotest.tools/v3/golden"
)

func TestTemplate(t *testing.T) {
	t.Run("ParseError", func(t *testing.T) {
		tmpl := code.MustTemplate(`this.IsNotValidGo("because it's missing a closing parenthesis"`, nil, context.GoLang{})
		stmt, err := tmpl.CompileBlock(mockAdviceContext{t})
		require.Nil(t, stmt)
		require.Error(t, err)
		golden.Assert(t, err.Error(), "parse_error.txt")
	})
}

type mockAdviceContext struct {
	t *testing.T
}

func (mockAdviceContext) ParseSource(src []byte) (*dst.File, error) {
	return decorator.Parse(src)
}

// The rest is not used by the tests as of now...
func (m mockAdviceContext) Release() {
	assert.FailNow(m.t, "unexpected method call")
}
func (m mockAdviceContext) Chain() *context.NodeChain {
	assert.FailNow(m.t, "unexpected method call")
	return nil
}
func (m mockAdviceContext) Node() dst.Node {
	assert.FailNow(m.t, "unexpected method call")
	return nil
}
func (m mockAdviceContext) Parent() context.AspectContext {
	assert.FailNow(m.t, "unexpected method call")
	return nil
}
func (m mockAdviceContext) Config(string) (string, bool) {
	assert.FailNow(m.t, "unexpected method call")
	return "", false
}
func (m mockAdviceContext) File() *dst.File {
	assert.FailNow(m.t, "unexpected method call")
	return nil
}
func (m mockAdviceContext) ImportPath() string {
	assert.FailNow(m.t, "unexpected method call")
	return ""
}
func (m mockAdviceContext) Package() string {
	assert.FailNow(m.t, "unexpected method call")
	return ""
}
func (m mockAdviceContext) Child(dst.Node, string, int) context.AdviceContext {
	assert.FailNow(m.t, "unexpected method call")
	return nil
}
func (m mockAdviceContext) ReplaceNode(dst.Node) {
	assert.FailNow(m.t, "unexpected method call")
}
func (m mockAdviceContext) AddImport(string, string) bool {
	assert.FailNow(m.t, "unexpected method call")
	return false
}
func (m mockAdviceContext) AddLink(string) bool {
	assert.FailNow(m.t, "unexpected method call")
	return false
}
func (m mockAdviceContext) EnsureMinGoLang(context.GoLang) {
	assert.FailNow(m.t, "unexpected method call")
}
