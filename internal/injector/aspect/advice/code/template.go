// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package code

import (
	"bytes"
	"errors"
	"fmt"
	"go/token"
	"strconv"
	"strings"
	"text/template"

	"github.com/dave/dst"
	"github.com/dave/dst/dstutil"
	"gopkg.in/yaml.v3"

	"github.com/DataDog/orchestrion/internal/fingerprint"
	"github.com/DataDog/orchestrion/internal/injector/aspect/context"
	"github.com/DataDog/orchestrion/internal/version"
)

type Template struct {
	template *template.Template
	Imports  map[string]string
	Source   string
	Lang     context.GoLangVersion
}

var wrapper = template.Must(template.New("code.Template").Funcs(template.FuncMap{
	"Version": version.Tag,
}).Parse(
	`
{{- define "_statements_" -}}
package _
func _() {
	{{ template "code.Template" . }}
}
{{- end -}}
{{- define "_declarations_" -}}
package _
{{ template "code.Template" . }}
{{- end -}}}
	`,
))

// NewTemplate creates a new Template using the provided template string and
// imports map. The imports map associates names to import paths. The produced
// AST nodes will feature qualified *dst.Ident nodes in all places where a
// property of mapped names is selected.
func NewTemplate(text string, imports map[string]string, lang context.GoLangVersion) (*Template, error) {
	template := template.Must(wrapper.Clone())
	template, err := template.Parse(text)
	return &Template{template, imports, text, lang}, err
}

// MustTemplate is the same as NewTemplate, but panics if an error occurs.
func MustTemplate(text string, imports map[string]string, lang context.GoLangVersion) (template *Template) {
	var err error
	if template, err = NewTemplate(text, imports, lang); err != nil {
		panic(err)
	}
	return
}

// CompileBlock generates new source based on this Template and wraps the
// resulting dst.Stmt nodes in a new *dst.BlockStmt. The provided
// context.Context and *dstutil.Cursor are used to supply context information to
// the template functions.
func (t *Template) CompileBlock(ctx context.AdviceContext) (*dst.BlockStmt, error) {
	stmts, err := t.compile(ctx)
	if err != nil {
		return nil, err
	}
	return &dst.BlockStmt{List: stmts}, nil
}

// CompileDeclarations generates new source based on this Template and extracts
// all produced declarations.
func (t *Template) CompileDeclarations(ctx context.AdviceContext) ([]dst.Decl, error) {
	res, err := t.compileTemplate(ctx, "_declarations_")
	if err != nil {
		return nil, fmt.Errorf("CompileDeclarations: %w", err)
	}
	return res, nil
}

// CompileExpression generates new source based on this Template and extracts
// the produced dst.Expr node. The provided context.Context and *dstutil.Cursor
// are used to supply context information to the template functions. The
// provided dst.Expr will be copied in places where the `{{Expr}}` template
// function is used, unless `expr` is nil.
func (t *Template) CompileExpression(ctx context.AdviceContext) (dst.Expr, error) {
	stmts, err := t.compile(ctx)
	if err != nil {
		return nil, fmt.Errorf("CompileExpression: %w", err)
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
func (t *Template) compile(ctx context.AdviceContext) ([]dst.Stmt, error) {
	decls, err := t.compileTemplate(ctx, "_statements_")
	if err != nil {
		return nil, err
	}

	return decls[0].(*dst.FuncDecl).Body.List, nil
}

func (t *Template) compileTemplate(ctx context.AdviceContext, name string) ([]dst.Decl, error) {
	tmpl := template.Must(t.template.Clone())

	buf := bytes.NewBuffer(nil)
	dot := &dot{context: ctx}
	if err := tmpl.ExecuteTemplate(buf, name, dot); err != nil {
		return nil, err
	}

	file, err := ctx.ParseSource(buf.Bytes())
	if err != nil {
		return nil, fmt.Errorf("while parsing generated code: %w\n%s", err, numberLines(buf.String()))
	}

	decls := make([]dst.Decl, 0, len(file.Decls))
	for _, decl := range file.Decls {
		if decl, ok := decl.(*dst.GenDecl); ok && decl.Tok == token.IMPORT {
			return nil, errors.New("code templates must not contain import declarations, use the imports map instead")
		}
		decls = append(decls, dot.placeholders.replaceAllIn(decl).(dst.Decl))
	}

	for i := range decls {
		decls[i] = t.processImports(ctx, decls[i])
	}

	return decls, nil
}

// processImports replaces all *dst.SelectorExpr based on one of the names
// present in the t.imports map with a qualified *dst.Ident node, so that the
// import-enabled decorator.Restorer can emit the correct code, and knows not to
// remove the inserted import statements.
func (t *Template) processImports(ctx context.AdviceContext, node dst.Decl) dst.Decl {
	if len(t.Imports) == 0 {
		return node
	}

	res, _ := dstutil.Apply(node, func(csor *dstutil.Cursor) bool {
		sel, ok := csor.Node().(*dst.SelectorExpr)
		if !ok {
			return true
		}

		ident, ok := sel.X.(*dst.Ident)
		if !ok {
			return true
		}

		path, found := t.Imports[ident.Name]
		if !found {
			return true
		}

		repl := sel.Sel
		repl.Path = path

		csor.Replace(repl)

		// We apply an alias to the import to mitigate the risk of conflicting with an existing symbol in the surrounding scope.
		ctx.AddImport(path, ident.Name)

		return true
	}, nil).(dst.Decl)

	return res
}

func (t *Template) Hash(h *fingerprint.Hasher) error {
	return h.Named(
		"template",
		fingerprint.Map(t.Imports, func(k string, v string) (string, fingerprint.String) { return k, fingerprint.String(v) }),
		fingerprint.String(t.Source),
		t.Lang,
	)
}

func (t *Template) AddedImports() []string {
	imports := make([]string, 0, len(t.Imports))
	for _, path := range t.Imports {
		imports = append(imports, path)
	}
	return imports
}

func (t *Template) UnmarshalYAML(node *yaml.Node) (err error) {
	var cfg struct {
		Template string
		Imports  map[string]string
		Links    []string
		Lang     context.GoLangVersion
	}
	if err = node.Decode(&cfg); err != nil {
		return
	}

	newT, err := NewTemplate(cfg.Template, cfg.Imports, cfg.Lang)
	if err != nil {
		return err
	}

	*t = *newT
	return nil
}

func numberLines(text string) string {
	lines := strings.Split(text, "\n")
	width := len(strconv.FormatInt(int64(len(lines)), 10))

	for i, line := range lines {
		lines[i] = fmt.Sprintf("% *d | %s", width+1, i+1, line)
	}

	return strings.Join(lines, "\n")
}
