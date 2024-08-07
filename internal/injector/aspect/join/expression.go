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

var funcNamePattern = regexp.MustCompile(`\A(?:(.+)\.)?([^.]+)\z`)

func init() {
	unmarshalers["function-call"] = func(node *yaml.Node) (Point, error) {
		var pattern string
		if err := node.Decode(&pattern); err != nil {
			return nil, err
		}
		return FunctionCall(pattern), nil
	}
}
