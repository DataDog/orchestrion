// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package code

import (
	"context"
	"errors"
	"fmt"
	"html/template"

	"github.com/datadog/orchestrion/internal/injector/typed"
	"github.com/dave/dst"
	"github.com/dave/dst/dstutil"
)

// templateFunctions returns a set of template function implementations. If expr
// is true, the `{{Expr}}` function is also made available.
func templateFunctions(ctx context.Context, csor *dstutil.Cursor, expr bool) template.FuncMap {
	result := template.FuncMap{
		"AssignedVar":     funcAssignedVariable(ctx, csor),
		"FuncArgName":     funcArgumentName(ctx, csor),
		"FuncName":        funcFunctionName(ctx, csor),
		"FuncReturnValue": funcReturnValue(ctx, csor),
	}

	if expr {
		result["Expr"] = func() string { return `_.Expr` }
	}

	return result
}

func funcAssignedVariable(ctx context.Context, csor *dstutil.Cursor) func() (string, error) {
	return func() (string, error) {
		assignStmt, ok := csor.Node().(*dst.AssignStmt)
		if !ok {
			return "", fmt.Errorf("not an assignment expression: %T", csor.Node())
		}

		if len(assignStmt.Lhs) != 1 {
			return "", fmt.Errorf("ambiguous reference to assigned variable, there are %d bindings. %w", len(assignStmt.Lhs), errors.ErrUnsupported)
		}

		ident, ok := assignStmt.Lhs[0].(*dst.Ident)
		if !ok {
			return "", fmt.Errorf("assigned variable is not an identifier: %T, %w", assignStmt.Lhs[0], errors.ErrUnsupported)
		}

		return ident.Name, nil
	}
}

func funcArgumentName(ctx context.Context, csor *dstutil.Cursor) func(int) (string, error) {
	return func(index int) (string, error) {
		funcType, err := getFuncType(ctx, csor)
		if err != nil {
			return "", err
		}
		if index >= len(funcType.Params.List) {
			return "", fmt.Errorf("requested name of parameter %d of function with %d parameters", index, len(funcType.Params.List))
		}

		// If arguments are anonymous, we'll proactively name all of them "_", and
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
	}
}

func funcFunctionName(ctx context.Context, csor *dstutil.Cursor) func() (string, error) {
	return func() (string, error) {
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
	}
}

func funcReturnValue(ctx context.Context, csor *dstutil.Cursor) func(int) (string, error) {
	return func(index int) (string, error) {
		funcType, err := getFuncType(ctx, csor)
		if err != nil {
			return "", err
		}

		if index >= len(funcType.Results.List) {
			return "", fmt.Errorf("requested name of return value %d of function with %d return values", index, len(funcType.Results.List))
		}

		// If return values are anonymous, proactively give them all referenceable
		// names.
		for i, res := range funcType.Results.List {
			if len(res.Names) == 0 {
				res.Names = []*dst.Ident{dst.NewIdent(fmt.Sprintf("_ret_%d", i))}
			}
		}

		ret := funcType.Results.List[index]
		if ret.Names[0].Name == "_" {
			ret.Names[0].Name = fmt.Sprintf("_ret_%d", index)
		}
		return ret.Names[0].Name, nil
	}
}

func getFuncType(ctx context.Context, csor *dstutil.Cursor) (*dst.FuncType, error) {
	if funcDecl, ok := typed.ContextValue[*dst.FuncDecl](ctx); ok {
		return funcDecl.Type, nil
	} else if funcDecl, ok = csor.Parent().(*dst.FuncDecl); ok {
		return funcDecl.Type, nil
	} else if funcLit, ok := csor.Parent().(*dst.FuncLit); ok {
		return funcLit.Type, nil
	} else {
		return nil, errors.New("no *dst.FuncDecl nor *dst.FuncLit is available in this context")
	}
}
