// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package join

import (
	"fmt"
	"regexp"

	"github.com/DataDog/orchestrion/internal/injector/aspect/context"
	"github.com/dave/dst"
	"github.com/dave/jennifer/jen"
	"gopkg.in/yaml.v3"
)

type functionCall struct {
	ImportPath string
	Name       string
}

func FunctionCall(pattern string) *functionCall {
	matches := funcNamePattern.FindStringSubmatch(pattern)
	if matches == nil {
		panic(fmt.Errorf("invalid function name pattern: %q", pattern))
	}

	return &functionCall{ImportPath: matches[1], Name: matches[2]}
}

func (i *functionCall) ImpliesImported() []string {
	return []string{i.ImportPath}
}

func (i *functionCall) Matches(ctx context.AspectContext) bool {
	call, ok := ctx.Node().(*dst.CallExpr)
	if !ok {
		return false
	}

	switch fun := call.Fun.(type) {
	case *dst.Ident:
		return fun.Path == i.ImportPath && fun.Name == i.Name
	case *dst.SelectorExpr:
		if fun.Sel.Name != i.Name {
			return false
		}
		ident, ok := fun.X.(*dst.Ident)
		if !ok {
			return false
		}

		// TODO: Must actually look at whether ident.Name is the import of the relevant package path.
		return ident.Path == i.ImportPath
	default:
		return false
	}
}

func (i *functionCall) AsCode() jen.Code {
	return jen.Qual(pkgPath, "FunctionCall").Call(jen.Lit(i.ImportPath + "." + i.Name))
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
