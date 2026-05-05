// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package join

import (
	gocontext "context"
	"fmt"
	"go/types"

	"github.com/DataDog/orchestrion/internal/fingerprint"
	"github.com/DataDog/orchestrion/internal/injector/aspect/context"
	"github.com/DataDog/orchestrion/internal/injector/aspect/may"
	"github.com/DataDog/orchestrion/internal/injector/typed"
	"github.com/DataDog/orchestrion/internal/yaml"
	"github.com/dave/dst"
	"github.com/goccy/go-yaml/ast"
)

type methodCall struct {
	Receiver typed.TypeName
	Name     string
}

func MethodCall(receiver typed.TypeName, name string) *methodCall {
	return &methodCall{Receiver: receiver, Name: name}
}

func (m *methodCall) ImpliesImported() []string {
	if path := m.Receiver.ImportPath; path != "" {
		return []string{path}
	}
	return nil
}

func (m *methodCall) PackageMayMatch(ctx *may.PackageContext) may.MatchType {
	return ctx.PackageImports(m.Receiver.ImportPath)
}

func (m *methodCall) FileMayMatch(ctx *may.FileContext) may.MatchType {
	return ctx.FileContains(m.Name)
}

func (m *methodCall) Matches(ctx context.AspectContext) bool {
	call, ok := ctx.Node().(*dst.CallExpr)
	if !ok {
		return false
	}

	selector, ok := call.Fun.(*dst.SelectorExpr)
	if !ok || selector.Sel.Name != m.Name {
		return false
	}

	recvType := ctx.ResolveType(selector.X)
	return m.matchesType(recvType)
}

// matchesType checks whether the resolved go/types.Type corresponds to the expected receiver.
func (m *methodCall) matchesType(t types.Type) bool {
	if t == nil {
		return false
	}

	if m.Receiver.Pointer {
		ptr, ok := t.(*types.Pointer)
		if !ok {
			return false
		}
		t = ptr.Elem()
	}

	named, ok := t.(*types.Named)
	if !ok {
		return false
	}

	obj := named.Obj()
	return obj.Pkg() != nil &&
		obj.Pkg().Path() == m.Receiver.ImportPath &&
		obj.Name() == m.Receiver.Name
}

func (m *methodCall) Hash(h *fingerprint.Hasher) error {
	return h.Named("method-call", m.Receiver, fingerprint.String(m.Name))
}

func init() {
	unmarshalers["method-call"] = func(ctx gocontext.Context, node ast.Node) (Point, error) {
		var spec struct {
			Receiver string `yaml:"receiver"`
			Name     string `yaml:"name"`
		}
		if err := yaml.NodeToValueContext(ctx, node, &spec); err != nil {
			return nil, err
		}

		if spec.Receiver == "" {
			return nil, fmt.Errorf("method-call: missing required field 'receiver'")
		}
		if spec.Name == "" {
			return nil, fmt.Errorf("method-call: missing required field 'name'")
		}

		tn, err := typed.NewTypeName(spec.Receiver)
		if err != nil {
			return nil, fmt.Errorf("method-call: invalid receiver type %q: %w", spec.Receiver, err)
		}

		return MethodCall(tn, spec.Name), nil
	}
}
