// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package join

import (
	gocontext "context"

	"github.com/DataDog/orchestrion/internal/fingerprint"
	"github.com/DataDog/orchestrion/internal/injector/aspect/context"
	"github.com/DataDog/orchestrion/internal/injector/typed"
	"github.com/dave/dst"
	"github.com/goccy/go-yaml/ast"
)

type methodValue struct {
	*methodCall
}

// MethodValue matches method selector expressions captured as values.
func MethodValue(receiver typed.TypeName, name string, match MethodCallMatch) *methodValue {
	return &methodValue{methodCall: MethodCall(receiver, name, match)}
}

func (m *methodValue) Matches(ctx context.AspectContext) bool {
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

	return m.matchesType(ctx.ResolveType(selector.X))
}

func (m *methodValue) Hash(h *fingerprint.Hasher) error {
	return h.Named("method-value", m.Receiver, fingerprint.String(m.Name), m.Match)
}

func init() {
	unmarshalers["method-value"] = func(ctx gocontext.Context, node ast.Node) (Point, error) {
		tn, name, match, err := unmarshalMethodSpec(ctx, node, "method-value")
		if err != nil {
			return nil, err
		}

		return MethodValue(tn, name, match), nil
	}
}
