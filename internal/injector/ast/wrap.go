// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package ast

import (
	"bytes"
	"context"
	"fmt"
	"text/template"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
	"github.com/dave/dst/dstutil"
	"gopkg.in/yaml.v3"
)

type wrapExpression struct {
	template string
}

func WrapExpression(template string) *wrapExpression {
	return &wrapExpression{template: template}
}

func (a *wrapExpression) Apply(ctx context.Context, csor *dstutil.Cursor) (bool, error) {
	var (
		expr dst.Expr
		kve  *dst.KeyValueExpr
		ok   bool
	)

	if kve, ok = csor.Node().(*dst.KeyValueExpr); ok {
		expr = kve.Value
	} else if expr, ok = csor.Node().(dst.Expr); !ok {
		return false, fmt.Errorf("expected dst.Expr or *dst.KeyValueExpr, got %T", csor.Node())
	}

	tmpl := template.New("_").Funcs(template.FuncMap{"Expr": func() string { return "_.Expr" }})
	tmpl, err := tmpl.Parse("{{define `Wrapper`}}package _\n\nfunc _(){\n{{template `_` .}}\n}{{end}}")
	if err != nil {
		return false, err
	}
	tmpl, err = tmpl.Parse(a.template)
	if err != nil {
		return false, err
	}

	buf := bytes.NewBuffer(make([]byte, 0, 4_096))
	if err := tmpl.ExecuteTemplate(buf, "Wrapper", expr); err != nil {
		return false, err
	}

	file, err := decorator.Parse(buf.Bytes())
	if err != nil {
		return false, err
	}
	stmts := file.Decls[0].(*dst.FuncDecl).Body.List
	if len(stmts) != 1 {
		return false, fmt.Errorf("wrap-expression template must produce a single expression (got %d)", len(stmts))
	}
	out, ok := stmts[0].(*dst.ExprStmt)
	if !ok {
		return false, fmt.Errorf("wrap-expression template must produce an expression (got %T)", stmts[0])
	}

	outX := out.X
	dstutil.Apply(outX, func(csor *dstutil.Cursor) bool {
		sel, ok := csor.Node().(*dst.SelectorExpr)
		if !ok {
			return true
		}
		if sel.Sel.Name != "Expr" {
			return true
		}
		ident, ok := sel.X.(*dst.Ident)
		if !ok {
			return true
		}
		if ident.Path != "" || ident.Name != "_" {
			return true
		}
		csor.Replace(expr)
		return false
	}, nil)

	// Move the decorations from the statement to the expression itself
	deco := outX.Decorations()
	deco.Before = dst.NewLine
	deco.Start = out.Decorations().Start
	deco.End = out.Decorations().End

	if kve == nil {
		csor.Replace(outX)
	} else {
		kve.Value = outX
	}

	return true, nil
}

func init() {
	unmarshalers["wrap-expression"] = func(node *yaml.Node) (Action, error) {
		var template string
		if err := node.Decode(&template); err != nil {
			return nil, err
		}
		return WrapExpression(template), nil
	}
}
