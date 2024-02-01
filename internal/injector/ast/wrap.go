// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package ast

import (
	"context"
	"fmt"

	"github.com/datadog/orchestrion/internal/injector/ast/code"
	"github.com/dave/dst"
	"github.com/dave/dst/dstutil"
	"gopkg.in/yaml.v3"
)

type wrapExpression struct {
	template code.Template
}

func WrapExpression(template code.Template) *wrapExpression {
	return &wrapExpression{template: template}
}

func (a *wrapExpression) Apply(ctx context.Context, csor *dstutil.Cursor) (bool, error) {
	var (
		expr dst.Expr
		kve  *dst.KeyValueExpr
		ok   bool
	)

	if kve, ok = csor.Node().(*dst.KeyValueExpr); ok {
		expr = kve.Value
	} else if expr, ok = csor.Node().(dst.Expr); !ok {
		return false, fmt.Errorf("expected dst.Expr or *dst.KeyValueExpr, got %T", csor.Node())
	}

	repl, err := a.template.CompileExpression(ctx, csor, expr)
	if err != nil {
		return false, err
	}

	if kve == nil {
		csor.Replace(repl)
	} else {
		kve.Value = repl
	}

	return true, nil
}

func init() {
	unmarshalers["wrap-expression"] = func(node *yaml.Node) (Action, error) {
		var template code.Template
		if err := node.Decode(&template); err != nil {
			return nil, err
		}
		return WrapExpression(template), nil
	}
}
