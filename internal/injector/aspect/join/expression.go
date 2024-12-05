// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package join

import (
	"fmt"
	"regexp"

	"github.com/DataDog/orchestrion/internal/fingerprint"
	"github.com/DataDog/orchestrion/internal/injector/aspect/context"
	"github.com/DataDog/orchestrion/internal/injector/aspect/may"
	"github.com/dave/dst"
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

func (i *functionCall) PackageMayMatch(ctx *may.PackageContext) may.MatchType {
	return ctx.PackageImports(i.ImportPath)
}

func (i *functionCall) FileMayMatch(ctx *may.FileMayMatchContext) may.MatchType {
	return ctx.FileContains(i.Name)
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

func (i *functionCall) Hash(h *fingerprint.Hasher) error {
	return h.Named("function-call", fingerprint.String(i.ImportPath), fingerprint.String(i.Name))
}

// See: https://regex101.com/r/fjLo1l/1
var funcNamePattern = regexp.MustCompile(`\A(?:(.+)\.)?([\p{L}_][\p{L}_\p{Nd}]*)\z`)

func init() {
	unmarshalers["function-call"] = func(node *yaml.Node) (Point, error) {
		var symbol string
		if err := node.Decode(&symbol); err != nil {
			return nil, err
		}

		matches := funcNamePattern.FindStringSubmatch(symbol)
		if matches == nil {
			return nil, fmt.Errorf("invalid function name %q", symbol)
		}

		return FunctionCall(matches[1], matches[2]), nil
	}
}
