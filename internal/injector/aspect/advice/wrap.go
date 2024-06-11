// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package advice

import (
	"context"
	"fmt"

	"github.com/datadog/orchestrion/internal/injector/aspect/advice/code"
	"github.com/datadog/orchestrion/internal/injector/node"
	"github.com/dave/dst"
	"github.com/dave/dst/dstutil"
	"github.com/dave/jennifer/jen"
	"gopkg.in/yaml.v3"
)

type wrapExpression struct {
	template code.Template
}

func WrapExpression(template code.Template) *wrapExpression {
	return &wrapExpression{template: template}
}

func (a *wrapExpression) Apply(ctx context.Context, node *node.Chain, csor *dstutil.Cursor) (bool, error) {
	var (
		kve *dst.KeyValueExpr
		ok  bool
	)

	if kve, ok = csor.Node().(*dst.KeyValueExpr); ok {
		node = node.Child(kve.Value, node.ImportPath(), "Value", -1)
	} else if _, ok = csor.Node().(dst.Expr); !ok {
		return false, fmt.Errorf("expected dst.Expr or *dst.KeyValueExpr, got %T", csor.Node())
	}

	repl, err := a.template.CompileExpression(ctx, node)
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

func (a *wrapExpression) AsCode() jen.Code {
	return jen.Qual(pkgPath, "WrapExpression").Call(a.template.AsCode())
}

func (a *wrapExpression) AddedImports() []string {
	return a.template.AddedImports()
}

func (a *wrapExpression) RenderHTML() string {
	return "wrap-expression"
}

func init() {
	unmarshalers["wrap-expression"] = func(node *yaml.Node) (Advice, error) {
		var template code.Template
		if err := node.Decode(&template); err != nil {
			return nil, err
		}
		return WrapExpression(template), nil
	}
}
