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

type (
	MethodCallMatch int

	methodCall struct {
		Receiver typed.TypeName
		Name     string
		Match    MethodCallMatch
	}
)

const (
	// MethodCallMatchAny matches calls regardless of whether the receiver is a pointer or value. This is the default.
	MethodCallMatchAny MethodCallMatch = iota
	// MethodCallMatchPointerOnly matches only calls where the receiver is a pointer type.
	MethodCallMatchPointerOnly
	// MethodCallMatchValueOnly matches only calls where the receiver is a value type.
	MethodCallMatchValueOnly
)

func MethodCall(receiver typed.TypeName, name string, match MethodCallMatch) *methodCall {
	return &methodCall{Receiver: receiver, Name: name, Match: match}
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

func (m *methodCall) matchesType(t types.Type) bool {
	if t == nil {
		return false
	}

	switch m.Match {
	case MethodCallMatchPointerOnly:
		ptr, ok := t.(*types.Pointer)
		if !ok {
			return false
		}
		return m.matchesNamed(ptr.Elem())
	case MethodCallMatchValueOnly:
		return m.matchesNamed(t)
	default: // MethodCallMatchAny
		if ptr, ok := t.(*types.Pointer); ok {
			t = ptr.Elem()
		}
		return m.matchesNamed(t)
	}
}

func (m *methodCall) matchesNamed(t types.Type) bool {
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
	return h.Named("method-call", m.Receiver, fingerprint.String(m.Name), m.Match)
}

func init() {
	unmarshalers["method-call"] = func(ctx gocontext.Context, node ast.Node) (Point, error) {
		var spec struct {
			Receiver string          `yaml:"receiver"`
			Name     string          `yaml:"name"`
			Match    MethodCallMatch `yaml:"match"`
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
		if tn.Pointer {
			return nil, fmt.Errorf("method-call: receiver type must not include a pointer sigil (use match: pointer-only instead): %q", spec.Receiver)
		}

		return MethodCall(tn, spec.Name, spec.Match), nil
	}
}

var _ yaml.NodeUnmarshalerContext = (*MethodCallMatch)(nil)

func (m *MethodCallMatch) UnmarshalYAML(ctx gocontext.Context, node ast.Node) error {
	var name string
	if err := yaml.NodeToValueContext(ctx, node, &name); err != nil {
		return err
	}

	switch name {
	case "any", "":
		*m = MethodCallMatchAny
	case "pointer-only":
		*m = MethodCallMatchPointerOnly
	case "value-only":
		*m = MethodCallMatchValueOnly
	default:
		return fmt.Errorf("invalid method-call.match value: %q", name)
	}

	return nil
}

func (m MethodCallMatch) String() string {
	switch m {
	case MethodCallMatchAny:
		return "any"
	case MethodCallMatchPointerOnly:
		return "pointer-only"
	case MethodCallMatchValueOnly:
		return "value-only"
	default:
		panic(fmt.Errorf("invalid MethodCallMatch(%d)", int(m)))
	}
}

func (m MethodCallMatch) Hash(h *fingerprint.Hasher) error {
	return h.Named("method-call-match", fingerprint.Int(m))
}
