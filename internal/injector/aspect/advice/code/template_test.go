// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package code_test

import (
	"context"
	"go/token"
	"testing"

	"github.com/datadog/orchestrion/internal/injector/aspect/advice/code"
	"github.com/datadog/orchestrion/internal/injector/node"
	"github.com/datadog/orchestrion/internal/injector/typed"
	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
	"github.com/stretchr/testify/require"
	"gotest.tools/v3/golden"
)

func TestTemplate(t *testing.T) {
	ctx := context.Background()
	ctx = typed.ContextWithValue(ctx, decorator.NewDecorator(token.NewFileSet()))

	t.Run("ParseError", func(t *testing.T) {
		tmpl := code.MustTemplate(`this.IsNotValidGo("because it's missing a closing parenthesis"`, nil)
		stmt, err := tmpl.CompileBlock(ctx, &node.Chain{Node: &dst.File{}})
		require.Nil(t, stmt)
		require.Error(t, err)
		golden.Assert(t, err.Error(), "parse_error.txt")
	})
}
