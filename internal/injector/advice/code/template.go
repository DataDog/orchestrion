// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package code

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"text/template"

	"github.com/datadog/orchestrion/internal/injector/node"
	"github.com/datadog/orchestrion/internal/injector/typed"
	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
	"github.com/dave/dst/dstutil"
	"gopkg.in/yaml.v3"
)

type Template struct {
	template *template.Template
	imports  map[string]string
}

var wrapper = template.Must(template.New("_").Parse("{{define `Wrapper`}}package _\n\nfunc _() {\n{{template `_` .}}\n}{{end}}"))

// NewTemplate creates a new Template using the provided template string and
// imports map. The imports map associates names to import paths. The produced
// AST nodes will feature qualified *dst.Ident nodes in all places where a
// property of mapped names is selected.
func NewTemplate(text string, imports map[string]string) (Template, error) {
	template := template.Must(wrapper.Clone())
	template, err := template.Parse(text)
	return Template{template, imports}, err
}

// CompileBlock generates new source based on this Template and wraps the
// resulting dst.Stmt nodes in a new *dst.BlockStmt. The provided
// context.Context and *dstutil.Cursor are used to supply context information to
// the template functions.
func (t *Template) CompileBlock(ctx context.Context, node *node.Chain) (*dst.BlockStmt, error) {
	stmts, err := t.compile(ctx, node, false)
	if err != nil {
		return nil, err
	}

	block := &dst.BlockStmt{List: stmts}
	block.Decs.Before = dst.NewLine
	block.Decs.Start.Prepend("//dd:startinstrument")
	block.Decs.End.Append("\n", "//dd:endinstrument")

	return block, nil
}

// CompileExpression generates new source based on this Template and extracts
// the produced dst.Expr node. The provided context.Context and *dstutil.Cursor
// are used to supply context information to the template functions. The
// provided dst.Expr will be copied in places where the `{{Expr}}` template
// function is used.
func (t *Template) CompileExpression(ctx context.Context, node *node.Chain, expr dst.Expr) (dst.Expr, error) {
	stmts, err := t.compile(ctx, node, true)
	if err != nil {
		return nil, err
	}

	if len(stmts) != 1 {
		return nil, fmt.Errorf("template must produce exactly 1 statement, but produced %d statements", len(stmts))
	}

	exprStmt, ok := stmts[0].(*dst.ExprStmt)
	if !ok {
		return nil, fmt.Errorf("template must produce an expression, but produced %T", stmts[0])
	}

	// Move the decorations from the statement to the expression itself.
	exprStmt.X.Decorations().Start = exprStmt.Decs.Start
	exprStmt.X.Decorations().End = exprStmt.Decs.End

	// Replace the _.Expr placeholder with the actual wrapped expression
	return dstutil.Apply(exprStmt.X, func(csor *dstutil.Cursor) bool {
		selectorExpr, ok := csor.Node().(*dst.SelectorExpr)
		if !ok {
			return true
		}
		if selectorExpr.Sel.Name != "Expr" {
			return true
		}
		ident, ok := selectorExpr.X.(*dst.Ident)
		if !ok {
			return true
		}
		if ident.Name == "_" {
			csor.Replace(expr)
		}
		return true
	}, nil).(dst.Expr), nil
}

// compile generates new source based on this Template and returns a cloned
// version of minimally post-processed dst.Stmt nodes this produced.
func (t *Template) compile(ctx context.Context, chain *node.Chain, hasExpression bool) ([]dst.Stmt, error) {
	ctxFile, found := node.Find[*dst.File](chain)
	if !found {
		return nil, errors.New("no *dst.File was found in the node chain")
	}

	tmpl := template.Must(t.template.Clone())

	buf := bytes.NewBuffer(nil)
	if err := tmpl.ExecuteTemplate(buf, "Wrapper", &dot{node: chain, hasExpression: hasExpression}); err != nil {
		return nil, err
	}

	dec, ok := typed.ContextValue[*decorator.Decorator](ctx)
	if !ok {
		return nil, errors.New("no *decorator.Decorator was available from context")
	}
	file, err := dec.Parse(buf.Bytes())
	if err != nil {
		return nil, err
	}

	body := file.Decls[0].(*dst.FuncDecl).Body.List
	stmts := make([]dst.Stmt, len(body))
	for i, node := range body {
		stmts[i] = dst.Clone(t.processImports(ctx, ctxFile, node)).(dst.Stmt)
	}
	return stmts, nil
}

// processImports replaces all *dst.SelectorExpr based on one of the names
// present in the t.imports map with a qualified *dst.Ident node, so that the
// import-enabled decorator.Restorer can emit the correct code, and knows not to
// remove the inserted import statements.
func (t *Template) processImports(ctx context.Context, file *dst.File, node dst.Node) dst.Node {
	if len(t.imports) == 0 {
		return node
	}

	return dstutil.Apply(node, func(csor *dstutil.Cursor) bool {
		sel, ok := csor.Node().(*dst.SelectorExpr)
		if !ok {
			return true
		}

		ident, ok := sel.X.(*dst.Ident)
		if !ok {
			return true
		}

		path, found := t.imports[ident.Name]
		if !found {
			return true
		}

		repl := sel.Sel
		repl.Path = path

		csor.Replace(repl)
		if refMap, ok := typed.ContextValue[*typed.ReferenceMap](ctx); ok {
			refMap.AddImport(file, path)
		}

		return true
	}, nil)
}

func (t *Template) UnmarshalYAML(node *yaml.Node) (err error) {
	var cfg struct {
		Template string
		Imports  map[string]string
	}
	if err = node.Decode(&cfg); err != nil {
		return
	}
	*t, err = NewTemplate(cfg.Template, cfg.Imports)
	return err
}
