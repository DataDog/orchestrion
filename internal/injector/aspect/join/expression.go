// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package join

import (
	"fmt"
	"regexp"

	"github.com/datadog/orchestrion/internal/injector/aspect/context"
	"github.com/dave/dst"
	"github.com/dave/jennifer/jen"
	"gopkg.in/yaml.v3"
)

type functionCall struct {
	path string
	name string
}

func FunctionCall(pattern string) *functionCall {
	matches := funcNamePattern.FindStringSubmatch(pattern)
	if matches == nil {
		panic(fmt.Errorf("invalid function name pattern: %q", pattern))
	}

	return &functionCall{path: matches[1], name: matches[2]}
}

func (i *functionCall) ImpliesImported() []string {
	return []string{i.path}
}

func (i *functionCall) Matches(ctx context.AspectContext) bool {
	call, ok := ctx.Node().(*dst.CallExpr)
	if !ok {
		return false
	}

	switch fun := call.Fun.(type) {
	case *dst.Ident:
		return fun.Path == i.path && fun.Name == i.name
	case *dst.SelectorExpr:
		if fun.Sel.Name != i.name {
			return false
		}
		ident, ok := fun.X.(*dst.Ident)
		if !ok {
			return false
		}

		// TODO: Must actually look at whether ident.Name is the import of the relevant package path.
		return ident.Path == i.path
	default:
		return false
	}
}

func (i *functionCall) AsCode() jen.Code {
	return jen.Qual(pkgPath, "FunctionCall").Call(jen.Lit(i.path + "." + i.name))
}

func (i *functionCall) RenderHTML() string {
	return fmt.Sprintf(`<div class="flex join-point function-call"><span class="type">Call to</span>{{<godoc %q %q>}}</div>`, i.path, i.name)
}

type methodCall struct {
	receiver TypeName
	name     string
}

func MethodCall(receiver TypeName, name string) *methodCall {
	return &methodCall{receiver, name}
}

func (i *methodCall) ImpliesImported() []string {
	if path := i.receiver.ImportPath(); path != "" {
		return []string{path}
	}
	return nil
}

func (i *methodCall) Matches(ctx context.AspectContext) bool {
	call, ok := ctx.Node().(*dst.CallExpr)
	if !ok {
		return false
	}

	selector, ok := call.Fun.(*dst.SelectorExpr)
	if !ok || selector.Sel.Name != i.name {
		return false
	}

	recvType := ctx.TypeOf(selector.X)
	if recvType == nil {
		return false
	}

	return i.receiver.matchesType(recvType)
}

func (i *methodCall) AsCode() jen.Code {
	return jen.Qual(pkgPath, "MethodCall").Call(
		i.receiver.AsCode(),
		jen.Lit(i.name),
	)
}

func (i *methodCall) RenderHTML() string {
	return fmt.Sprintf(`<div class="flex join-point method-call"><span class="type">Call to</span>%s.%s</div>`, i.receiver.RenderHTML(), i.name)
}

var funcNamePattern = regexp.MustCompile(`\A(?:(.+)\.)?([^.]+)\z`)

func init() {
	unmarshalers["function-call"] = func(node *yaml.Node) (Point, error) {
		var pattern string
		if err := node.Decode(&pattern); err != nil {
			return nil, err
		}
		return FunctionCall(pattern), nil
	}

	unmarshalers["method-call"] = func(node *yaml.Node) (Point, error) {
		var opts struct {
			Receiver string
			Name     string
		}
		if err := node.Decode(&opts); err != nil {
			return nil, err
		}

		receiver, err := NewTypeName(opts.Receiver)
		if err != nil {
			return nil, err
		}

		return MethodCall(receiver, opts.Name), nil
	}
}
