// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package advice

import (
	gocontext "context"
	"fmt"
	"regexp"
	"strings"

	"github.com/DataDog/orchestrion/internal/fingerprint"
	"github.com/DataDog/orchestrion/internal/injector/aspect/advice/code"
	"github.com/DataDog/orchestrion/internal/injector/aspect/context"
	"github.com/DataDog/orchestrion/internal/injector/typed"
	"github.com/DataDog/orchestrion/internal/yaml"
	"github.com/dave/dst"
	"github.com/goccy/go-yaml/ast"
)

type appendArgs struct {
	TypeName  *typed.NamedType
	Templates []*code.Template
}

// AppendArgs appends arguments of a given type to the end of a function call. All arguments must be
// of the same type, as they may be appended at the tail end of a variadic call.
func AppendArgs(typeName *typed.NamedType, templates ...*code.Template) *appendArgs {
	return &appendArgs{typeName, templates}
}

func (a *appendArgs) Apply(ctx context.AdviceContext) (bool, error) {
	call, ok := ctx.Node().(*dst.CallExpr)
	if !ok {
		return false, fmt.Errorf("append-arguments: expected a *dst.CallExpr, received %T", ctx.Node())
	}

	newArgs := make([]dst.Expr, len(a.Templates))
	var err error
	for i, t := range a.Templates {
		newArgs[i], err = t.CompileExpression(ctx)
		if err != nil {
			return false, fmt.Errorf("append-arguments[%d]: %w", i, err)
		}
		ctx.EnsureMinGoLang(t.Lang)
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
						Type:  &dst.Ellipsis{Elt: a.TypeName.AsNode()},
					}},
				},
				Results: &dst.FieldList{
					List: []*dst.Field{{
						Type: &dst.ArrayType{Elt: a.TypeName.AsNode()},
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

	if importPath := a.TypeName.ImportPath; importPath != "" {
		ctx.AddImport(importPath, inferPkgName(importPath))
	}

	return true, nil
}

func (a *appendArgs) AddedImports() []string {
	imports := make([]string, 0, len(a.Templates)+1)
	if argTypeImportPath := a.TypeName.ImportPath; argTypeImportPath != "" {
		imports = append(imports, argTypeImportPath)
	}
	for _, t := range a.Templates {
		imports = append(imports, t.AddedImports()...)
	}
	return imports
}

func (a *appendArgs) Hash(h *fingerprint.Hasher) error {
	return h.Named("append-args", a.TypeName, fingerprint.List[*code.Template](a.Templates))
}

type redirectCall struct {
	ImportPath string
	Name       string
}

// ReplaceFunction replaces the called function with the provided drop-in replacement. The signature
// must be compatible with the original function (it may accept a new variadic argument).
func ReplaceFunction(path string, name string) *redirectCall {
	return &redirectCall{path, name}
}

func (r *redirectCall) Apply(ctx context.AdviceContext) (bool, error) {
	node, ok := ctx.Node().(*dst.CallExpr)
	if !ok {
		return false, fmt.Errorf("expected a *dst.CallExpr, received %T", ctx.Node())
	}

	if id, ok := node.Fun.(*dst.Ident); ok {
		id.Path = r.ImportPath
		id.Name = r.Name
		id.Obj = nil // Just in case
	} else {
		node.Fun = &dst.Ident{Path: r.ImportPath, Name: r.Name}
	}

	if r.ImportPath != "" {
		ctx.AddImport(r.ImportPath, inferPkgName(r.ImportPath))
	}

	return true, nil
}

func (r *redirectCall) Hash(h *fingerprint.Hasher) error {
	return h.Named("replace-function", fingerprint.String(r.ImportPath), fingerprint.String(r.Name))
}

func (r *redirectCall) AddedImports() []string {
	if r.ImportPath != "" {
		return []string{r.ImportPath}
	}
	return nil
}

func init() {
	unmarshalers["append-args"] = func(ctx gocontext.Context, node ast.Node) (Advice, error) {
		var args struct {
			TypeName string           `yaml:"type"`
			Values   []*code.Template `yaml:"values"`
		}

		if err := yaml.NodeToValueContext(ctx, node, &args); err != nil {
			return nil, err
		}

		namedType, err := typed.NewNamedType(args.TypeName)
		if err != nil {
			return nil, fmt.Errorf("invalid type %q: %w", args.TypeName, err)
		}

		return AppendArgs(namedType, args.Values...), nil
	}
	unmarshalers["replace-function"] = func(ctx gocontext.Context, node ast.Node) (Advice, error) {
		var (
			fqn  string
			path string
			name string
		)
		if err := yaml.NodeToValueContext(ctx, node, &fqn); err != nil {
			return nil, err
		}

		if idx := strings.LastIndex(fqn, "."); idx >= 0 {
			path = fqn[:idx]
			name = fqn[idx+1:]
		} else {
			name = fqn // Built-in function, function from the same package
		}

		return ReplaceFunction(path, name), nil
	}
}

var (
	importPathRe        = regexp.MustCompile(`^(?:.+/)?([^/]+?)(?:\.v\d+|/v\d+)?$`)
	notValidIdentCharRe = regexp.MustCompile(`[^a-zA-Z0-9_]`)
)

// inferPkgName extracts the last part of an import path and sanitizes it to be a valid Go
// identifier continuation (meaning it assumes it would be appended to a valid Go identifier). This
// is done for cosmetic purposes and does not require being accurate.
func inferPkgName(importPath string) string {
	matches := importPathRe.FindStringSubmatch(importPath)
	var pkgName string
	if len(matches) < 2 {
		pkgName = importPath
	} else {
		pkgName = matches[1]
	}
	return notValidIdentCharRe.ReplaceAllLiteralString(pkgName, "_")
}
