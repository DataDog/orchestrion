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
	"sort"
	"strconv"
	"strings"
	"text/template"

	"github.com/datadog/orchestrion/internal/injector/node"
	"github.com/datadog/orchestrion/internal/injector/typed"
	"github.com/datadog/orchestrion/internal/version"
	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
	"github.com/dave/dst/dstutil"
	"github.com/dave/jennifer/jen"
	"gopkg.in/yaml.v3"
)

type Template struct {
	template *template.Template
	imports  map[string]string
}

var wrapper = template.Must(template.New("code.Template").Funcs(template.FuncMap{
	"Version": func() string { return version.Tag },
}).Parse(
	"{{define `_`}}package _\nfunc _() {\n{{template `code.Template` .}}\n}{{end}}",
))

// NewTemplate creates a new Template using the provided template string and
// imports map. The imports map associates names to import paths. The produced
// AST nodes will feature qualified *dst.Ident nodes in all places where a
// property of mapped names is selected.
func NewTemplate(text string, imports map[string]string) (Template, error) {
	template := template.Must(wrapper.Clone())
	template, err := template.Parse(text)
	return Template{template, imports}, err
}

// MustTemplate is the same as NewTemplate, but panics if an error occurs.
func MustTemplate(text string, imports map[string]string) (template Template) {
	var err error
	if template, err = NewTemplate(text, imports); err != nil {
		panic(err)
	}
	return
}

// CompileBlock generates new source based on this Template and wraps the
// resulting dst.Stmt nodes in a new *dst.BlockStmt. The provided
// context.Context and *dstutil.Cursor are used to supply context information to
// the template functions.
func (t *Template) CompileBlock(ctx context.Context, node *node.Chain) (*dst.BlockStmt, error) {
	stmts, err := t.compile(ctx, node)
	if err != nil {
		return nil, err
	}
	return &dst.BlockStmt{List: stmts}, nil
}

// CompileExpression generates new source based on this Template and extracts
// the produced dst.Expr node. The provided context.Context and *dstutil.Cursor
// are used to supply context information to the template functions. The
// provided dst.Expr will be copied in places where the `{{Expr}}` template
// function is used, unless `expr` is nil.
func (t *Template) CompileExpression(ctx context.Context, node *node.Chain) (dst.Expr, error) {
	stmts, err := t.compile(ctx, node)
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

	result := exprStmt.X
	// Move the decorations from the statement to the expression itself.
	result.Decorations().Start = exprStmt.Decs.Start
	result.Decorations().End = exprStmt.Decs.End

	return result, nil
}

// compile generates new source based on this Template and returns a cloned
// version of minimally post-processed dst.Stmt nodes this produced.
func (t *Template) compile(ctx context.Context, chain *node.Chain) ([]dst.Stmt, error) {
	ctxFile, found := node.Find[*dst.File](chain)
	if !found {
		return nil, errors.New("no *dst.File was found in the node chain")
	}

	tmpl := template.Must(t.template.Clone())

	buf := bytes.NewBuffer(nil)
	dot := &dot{node: chain}
	if err := tmpl.ExecuteTemplate(buf, "_", dot); err != nil {
		return nil, fmt.Errorf("while executing template: %w", err)
	}

	dec, ok := typed.ContextValue[*decorator.Decorator](ctx)
	if !ok {
		return nil, errors.New("no *decorator.Decorator was available from context")
	}
	file, err := dec.Parse(buf.Bytes())
	if err != nil {
		return nil, fmt.Errorf("while parsing generated code: %w\n%q", err, numberLines(buf.String()))
	}

	body := file.Decls[0].(*dst.FuncDecl).Body
	dot.placeholders.replaceAllIn(body)

	list := body.List
	stmts := make([]dst.Stmt, len(list))
	for i, node := range list {
		stmts[i] = t.processImports(ctx, ctxFile, node).(dst.Stmt)
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

func (t *Template) AsCode() jen.Code {
	return jen.Qual("github.com/datadog/orchestrion/internal/injector/aspect/advice/code", "MustTemplate").Call(
		jen.Line().Lit(t.template.Tree.Root.String()),
		jen.Line().Map(jen.String()).String().ValuesFunc(func(g *jen.Group) {
			// We sort the keys so the generated code order is consistent...
			keys := make([]string, 0, len(t.imports))
			for k := range t.imports {
				keys = append(keys, k)
			}
			sort.Strings(keys)

			for _, k := range keys {
				v := t.imports[k]
				g.Line().Add(jen.Lit(k).Op(":").Lit(v))
			}
			g.Empty().Line()
		}),
		jen.Empty().Line(),
	)
}

func (t *Template) AddedImports() []string {
	imports := make([]string, 0, len(t.imports))
	for _, path := range t.imports {
		imports = append(imports, path)
	}
	return imports
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

func numberLines(text string) string {
	lines := strings.Split(text, "\n")
	width := len(strconv.FormatInt(int64(len(lines)), 10))
	format := fmt.Sprintf("%% %dd | %%s", width)

	for i, line := range lines {
		lines[i] = fmt.Sprintf(format, i+1, line)
	}

	return strings.Join(lines, "\n")
}
