// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package advice

import (
	"context"
	"fmt"

	"github.com/datadog/orchestrion/internal/injector/aspect/advice/code"
	"github.com/datadog/orchestrion/internal/injector/aspect/join"
	"github.com/datadog/orchestrion/internal/injector/node"
	"github.com/datadog/orchestrion/internal/injector/typed"
	"github.com/dave/dst"
	"github.com/dave/dst/dstutil"
	"github.com/dave/jennifer/jen"
	"gopkg.in/yaml.v3"
)

type appendArgs struct {
	typeName  join.TypeName
	templates []code.Template
}

// AppendArgs appends arguments of a given type to the end of a function call. All arguments must be
// of the same type, as they may be appended at the tail end of a variadic call.
func AppendArgs(typeName join.TypeName, templates ...code.Template) *appendArgs {
	return &appendArgs{typeName, templates}
}

func (a *appendArgs) Apply(ctx context.Context, chain *node.Chain, csor *dstutil.Cursor) (bool, error) {
	call, ok := chain.Node.(*dst.CallExpr)
	if !ok {
		return false, fmt.Errorf("expected a *dst.CallExpr, received %T", chain.Node)
	}

	newArgs := make([]dst.Expr, len(a.templates))
	var err error
	for i, t := range a.templates {
		newArgs[i], err = t.CompileExpression(ctx, chain)
		if err != nil {
			return false, err
		}
	}

	if !call.Ellipsis {
		call.Args = append(call.Args, newArgs...)
		return true, nil
	}

	// The function call has an ellipsis, so we need to append our new arguments to the last argument,
	// which is a slice. To do so, we need to provision a new slice of the right type and size, append
	// all the relevant data in there, and then replace the last argument with the new slice.
	lastIdx := len(call.Args) - 1
	call.Args[lastIdx] = &dst.CallExpr{
		Fun: &dst.FuncLit{
			Type: &dst.FuncType{
				Params: &dst.FieldList{
					List: []*dst.Field{{
						Names: []*dst.Ident{dst.NewIdent("opts")},
						Type:  &dst.Ellipsis{Elt: a.typeName.AsNode()},
					}},
				},
				Results: &dst.FieldList{
					List: []*dst.Field{{
						Type: &dst.ArrayType{Elt: a.typeName.AsNode()},
					}},
				},
			},
			Body: &dst.BlockStmt{
				List: []dst.Stmt{
					&dst.ReturnStmt{Results: []dst.Expr{
						&dst.CallExpr{
							Fun: dst.NewIdent("append"),
							Args: append(
								append(
									make([]dst.Expr, 0, len(newArgs)+1),
									dst.NewIdent("opts"),
								),
								newArgs...,
							),
						},
					}},
				},
			},
		},
		Args:     []dst.Expr{call.Args[lastIdx]},
		Ellipsis: true,
	}

	if importPath := a.typeName.ImportPath(); importPath != "" {
		if file, ok := typed.ContextValue[*dst.File](ctx); ok {
			if refMap, ok := typed.ContextValue[*typed.ReferenceMap](ctx); ok {
				refMap.AddImport(file, importPath)
			}
		}
	}

	return true, nil
}

func (a *appendArgs) AsCode() jen.Code {
	return jen.Qual(pkgPath, "AppendArgs").CallFunc(func(group *jen.Group) {
		group.Line().Add(a.typeName.AsCode())
		for _, t := range a.templates {
			group.Line().Add(t.AsCode())
		}
		group.Empty().Line()
	})
}

func init() {
	unmarshalers["append-args"] = func(node *yaml.Node) (Advice, error) {
		var args struct {
			TypeName string          `yaml:"type"`
			Values   []code.Template `yaml:"values"`
		}

		if err := node.Decode(&args); err != nil {
			return nil, err
		}

		tn, err := join.NewTypeName(args.TypeName)
		if err != nil {
			return nil, err
		}

		return AppendArgs(tn, args.Values...), nil
	}
}
