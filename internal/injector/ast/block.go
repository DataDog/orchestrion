// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package ast

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"text/template"

	"github.com/datadog/orchestrion/internal/injector/typed"
	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
	"github.com/dave/dst/dstutil"
	"gopkg.in/yaml.v3"
)

type blockStmts struct {
	template string
}

// PrependStmts prepends statements to the matched *dst.BlockStmt. This action
// can only be used if the selector matches on a *dst.BlockStmt. The prepended
// statements are wrapped in a new block statement to prevent scope leakage.
func PrependStmts(text string) *blockStmts {
	return &blockStmts{template: text}
}

func (a *blockStmts) Apply(ctx context.Context, csor *dstutil.Cursor) (bool, error) {
	node := csor.Node()

	block, ok := node.(*dst.BlockStmt)
	if !ok {
		return false, fmt.Errorf("expected *dst.BlockStmt, got %T", node)
	}

	stmts, err := a.compile(ctx, csor)
	if err != nil {
		return false, err
	}

	list := make([]dst.Stmt, 1+len(block.List))
	list[0] = stmts
	copy(list[1:], block.List)
	block.List = list

	return true, nil
}

func (a *blockStmts) compile(ctx context.Context, csor *dstutil.Cursor) (*dst.BlockStmt, error) {
	tmpl := template.New("_").Funcs(templateFuncs(ctx, csor))
	tmpl, err := tmpl.Parse("{{define `Wrapper`}}package _\n\nfunc _(){\n{{template `_` .}}\n}{{end}}")
	if err != nil {
		panic(err)
	}
	tmpl, err = tmpl.Parse(a.template)
	if err != nil {
		panic(err)
	}

	buf := bytes.NewBuffer(make([]byte, 0, 1024))
	if err := tmpl.ExecuteTemplate(buf, "Wrapper", resolver{ctx}); err != nil {
		return nil, err
	}

	file, err := decorator.Parse(buf.Bytes())
	if err != nil {
		return nil, err
	}

	stmts := make([]dst.Stmt, len(file.Decls[0].(*dst.FuncDecl).Body.List))
	for i, node := range file.Decls[0].(*dst.FuncDecl).Body.List {
		stmts[i] = dst.Clone(node).(dst.Stmt)
	}

	block := &dst.BlockStmt{List: stmts}
	block.Decs.Before = dst.NewLine
	block.Decs.Start.Prepend("//dd:startinstrument")
	block.Decs.End.Append("\n", "//dd:endinstrument")

	return block, nil
}

type resolver struct {
	context.Context
}

func (r resolver) FuncDecl() (*dst.FuncDecl, error) {
	val, ok := typed.ContextValue[*dst.FuncDecl](r)
	if !ok {
		return nil, errors.New("no *dst.FuncDecl is available from context")
	}
	return val, nil
}

func init() {
	unmarshalers["prepend-statements"] = func(node *yaml.Node) (Action, error) {
		var code string
		if err := node.Decode(&code); err != nil {
			return nil, err
		}

		return PrependStmts(code), nil
	}
}
