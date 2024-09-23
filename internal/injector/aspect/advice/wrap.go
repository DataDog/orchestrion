// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package advice

import (
	"fmt"

	"github.com/DataDog/orchestrion/internal/injector/aspect/advice/code"
	"github.com/DataDog/orchestrion/internal/injector/aspect/context"
	"github.com/dave/dst"
	"github.com/dave/jennifer/jen"
	"gopkg.in/yaml.v3"
)

type wrapExpression struct {
	template code.Template
}

func WrapExpression(template code.Template) *wrapExpression {
	return &wrapExpression{template: template}
}

func (a *wrapExpression) Apply(ctx context.AdviceContext) (bool, error) {
	var (
		kve *dst.KeyValueExpr
		ok  bool
	)

	if kve, ok = ctx.Node().(*dst.KeyValueExpr); ok {
		ctx = ctx.Child(kve.Value, "Value", -1)
	} else if _, ok = ctx.Node().(dst.Expr); !ok {
		return false, fmt.Errorf("expected dst.Expr or *dst.KeyValueExpr, got %T", ctx.Node())
	}

	repl, err := a.template.CompileExpression(ctx)
	if err != nil {
		return false, err
	}

	if kve == nil {
		ctx.ReplaceNode(repl)
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
	return fmt.Sprintf(`<div class="advice wrap-expression"><div class="type">Replace the expression using the template:</div>%s</div>`, a.template.RenderHTML())
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
