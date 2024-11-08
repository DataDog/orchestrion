// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package join

import (
	"fmt"

	"github.com/DataDog/orchestrion/internal/injector/aspect/context"
	"github.com/dave/dst"
	"github.com/dave/jennifer/jen"
	"github.com/dlclark/regexp2"
	"gopkg.in/yaml.v3"
)

type functionCall struct {
	ImportPath string
	Name       string
}

func FunctionCall(importPath string, name string) *functionCall {
	return &functionCall{ImportPath: importPath, Name: name}
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
	return jen.Qual(pkgPath, "FunctionCall").Call(jen.Lit(i.ImportPath), jen.Lit(i.Name))
}

// See: https://regex101.com/r/fjLo1l/1
var funcNamePattern = regexp2.MustCompile(`\A(?:(.+)\.)?([\p{L}_][\p{L}_\p{Nd}]*)\z`, regexp2.ECMAScript)

func init() {
	unmarshalers["function-call"] = func(node *yaml.Node) (Point, error) {
		var symbol string
		if err := node.Decode(&symbol); err != nil {
			return nil, err
		}

		matches, err := funcNamePattern.FindStringMatch(symbol)
		if err != nil {
			return nil, fmt.Errorf("invalid function name %q: %w", symbol, err)
		}

		importPath := matches.GroupByNumber(1).String()
		name := matches.GroupByNumber(2).String()

		return FunctionCall(importPath, name), nil
	}
}
