// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package ast

import (
	"context"
	"errors"
	"fmt"
	"text/template"

	"github.com/datadog/orchestrion/internal/injector/typed"
	"github.com/dave/dst"
	"github.com/dave/dst/dstutil"
)

func templateFuncs(ctx context.Context, csor *dstutil.Cursor) template.FuncMap {
	return template.FuncMap{
		"FuncArgName": func(index int) (string, error) {
			var funcType *dst.FuncType
			if funcDecl, ok := typed.ContextValue[*dst.FuncDecl](ctx); ok {
				funcType = funcDecl.Type
			} else if funcDecl, ok = csor.Parent().(*dst.FuncDecl); ok {
				funcType = funcDecl.Type
			} else if funcLit, ok := csor.Parent().(*dst.FuncLit); ok {
				funcType = funcLit.Type
			} else {
				return "", errors.New("no *dst.FuncDecl nor *dst.FuncLit is available in this context")
			}
			if index >= len(funcType.Params.List) {
				return "", fmt.Errorf("requested name of parameter %d of function with %d parameters", index, len(funcType.Params.List))
			}

			// Is arguments are anonymous, we'll proactively name all of them "_", and
			// the arguments that are actually used (their name is returned by this
			// funtion) will be named `_<index>`.
			for _, param := range funcType.Params.List {
				if len(param.Names) == 0 {
					param.Names = []*dst.Ident{dst.NewIdent("_")}
				}
			}

			names := funcType.Params.List[index].Names
			if names[0].Name == "_" {
				// Give a referenceable name to the argument instead of blank.
				names[0].Name = fmt.Sprintf("_%d", index)
			}
			return names[0].Name, nil
		},
		"FuncName": func() (string, error) {
			funcDecl, ok := typed.ContextValue[*dst.FuncDecl](ctx)
			if !ok {
				funcDecl, ok = csor.Parent().(*dst.FuncDecl)
			}
			if !ok {
				if _, ok := csor.Parent().(*dst.FuncLit); ok {
					// Function literals have no name, so we return an empty string.
					return "", nil
				}
			}
			if !ok {
				return "", errors.New("no *dst.FuncDecl or *dst.FuncType is available in this context")
			}
			return funcDecl.Name.Name, nil
		},
	}
}
