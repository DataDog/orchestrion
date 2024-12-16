// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package advice

import (
	"fmt"

	"github.com/DataDog/orchestrion/internal/fingerprint"
	"github.com/DataDog/orchestrion/internal/injector/aspect/advice/code"
	"github.com/DataDog/orchestrion/internal/injector/aspect/context"
	"github.com/dave/dst"
	"gopkg.in/yaml.v3"
)

type assignValue struct {
	Template *code.Template
}

func AssignValue(template *code.Template) *assignValue {
	return &assignValue{template}
}

func (a *assignValue) Apply(ctx context.AdviceContext) (bool, error) {
	spec, ok := ctx.Node().(*dst.ValueSpec)
	if !ok {
		return false, fmt.Errorf("assign-value: expected *dst.ValueSpec, got %T", ctx.Node())
	}

	expr, err := a.Template.CompileExpression(ctx)
	if err != nil {
		return false, fmt.Errorf("assign-value: %w", err)
	}

	spec.Values = make([]dst.Expr, len(spec.Names))
	for i := range spec.Values {
		spec.Values[i], _ = dst.Clone(expr).(dst.Expr)
	}

	ctx.EnsureMinGoLang(a.Template.Lang)

	return true, nil
}

func (a *assignValue) AddedImports() []string {
	return a.Template.AddedImports()
}

func (a *assignValue) Hash(h *fingerprint.Hasher) error {
	return h.Named("assign-value", a.Template)
}

func init() {
	unmarshalers["assign-value"] = func(node *yaml.Node) (Advice, error) {
		var template *code.Template
		if err := node.Decode(&template); err != nil {
			return nil, err
		}
		return AssignValue(template), nil
	}
}
