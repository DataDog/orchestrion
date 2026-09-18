// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package join

import (
	gocontext "context"
	"go/types"

	"github.com/DataDog/orchestrion/internal/fingerprint"
	"github.com/DataDog/orchestrion/internal/injector/aspect/context"
	"github.com/DataDog/orchestrion/internal/injector/typed"
	"github.com/dave/dst"
	"github.com/goccy/go-yaml/ast"
)

type methodExpression struct {
	*methodCall
}

// MethodExpression matches method selector expressions captured as function values.
func MethodExpression(receiver typed.TypeName, name string, match MethodCallMatch) *methodExpression {
	return &methodExpression{methodCall: MethodCall(receiver, name, match)}
}

func (m *methodExpression) Matches(ctx context.AspectContext) bool {
	selector, ok := ctx.Node().(*dst.SelectorExpr)
	if !ok || selector.Sel.Name != m.Name {
		return false
	}

	parent := ctx.Parent()
	if parent != nil {
		defer parent.Release()
		if call, ok := parent.Node().(*dst.CallExpr); ok && call.Fun == selector {
			return false
		}
	}

	selection := ctx.Selection(selector)
	return selection != nil && selection.Kind() == types.MethodExpr && m.matchesType(selection.Recv())
}

func (m *methodExpression) Hash(h *fingerprint.Hasher) error {
	return h.Named("method-expression", m.Receiver, fingerprint.String(m.Name), m.Match)
}

func init() {
	unmarshalers["method-expression"] = func(ctx gocontext.Context, node ast.Node) (Point, error) {
		tn, name, match, err := unmarshalMethodSpec(ctx, node, "method-expression")
		if err != nil {
			return nil, err
		}

		return MethodExpression(tn, name, match), nil
	}
}
