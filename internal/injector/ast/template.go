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
			funcDecl, ok := typed.ContextValue[*dst.FuncDecl](ctx)
			if !ok {
				funcDecl, ok = csor.Parent().(*dst.FuncDecl)
			}
			if !ok {
				return "", errors.New("no *dst.FuncDecl is available in this context")
			}
			if index >= len(funcDecl.Type.Params.List) {
				return "", fmt.Errorf("requested name of parameter %d of function with %d parameters", index, len(funcDecl.Type.Params.List))
			}

			// Is arguments are anonymous, we'll proactively name all of them "_", and
			// the arguments that are actually used (their name is returned by this
			// funtion) will be named `_<index>`.
			for _, param := range funcDecl.Type.Params.List {
				if len(param.Names) == 0 {
					param.Names = []*dst.Ident{dst.NewIdent("_")}
				}
			}

			names := funcDecl.Type.Params.List[index].Names
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
				return "", errors.New("no *dst.FuncDecl is available in this context")
			}
			return fmt.Sprintf("%q", funcDecl.Name.Name), nil
		},
	}
}
