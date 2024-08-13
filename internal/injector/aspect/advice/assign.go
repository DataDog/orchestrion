// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package advice

import (
	"fmt"

	"github.com/datadog/orchestrion/internal/injector/aspect/advice/code"
	"github.com/datadog/orchestrion/internal/injector/aspect/context"
	"github.com/dave/dst"
	"github.com/dave/jennifer/jen"
	"gopkg.in/yaml.v3"
)

type assignValue struct {
	template code.Template
}

func AssignValue(template code.Template) *assignValue {
	return &assignValue{template}
}

func (a *assignValue) Apply(ctx context.AdviceContext) (bool, error) {
	spec, ok := ctx.Node().(*dst.ValueSpec)
	if !ok {
		return false, fmt.Errorf("expected *dst.ValueSpec, got %T", ctx.Node())
	}

	expr, err := a.template.CompileExpression(ctx)
	if err != nil {
		return false, err
	}

	spec.Values = make([]dst.Expr, len(spec.Names))
	for i := range spec.Values {
		spec.Values[i] = dst.Clone(expr).(dst.Expr)
	}

	return true, nil
}

func (a *assignValue) AddedImports() []string {
	return a.template.AddedImports()
}

func (a *assignValue) AsCode() jen.Code {
	return jen.Qual(pkgPath, "AssignValue").Call(a.template.AsCode())
}

func (a *assignValue) RenderHTML() string {
	return fmt.Sprintf(`<div class="advice assign-value"><div class="type">Set initial value to:</div>%s</div>`, a.template.RenderHTML())
}

func init() {
	unmarshalers["assign-value"] = func(node *yaml.Node) (Advice, error) {
		var template code.Template
		if err := node.Decode(&template); err != nil {
			return nil, err
		}
		return AssignValue(template), nil
	}
}
