// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package advice

import (
	gocontext "context"
	"fmt"
	"go/types"
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
	TypeName  typed.TypeName
	Templates []*code.Template
}

// AppendArgs appends arguments of a given type to the end of a function call. All arguments must be
// of the same type, as they may be appended at the tail end of a variadic call.
func AppendArgs(typeName typed.TypeName, templates ...*code.Template) *appendArgs {
	return &appendArgs{typeName, templates}
}

func (a *appendArgs) Apply(ctx context.AdviceContext) (bool, error) {
	call, ok := ctx.Node().(*dst.CallExpr)
	if !ok {
		return false, fmt.Errorf("append-arguments: expected a *dst.CallExpr, received %T", ctx.Node())
	}

	if len(a.Templates) == 0 {
		// A zero-template advice changes nothing: the call, its imports, and its
		// language requirement are left untouched, whatever the call form is.
		return false, nil
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
		// f(g()) where g returns several values is valid Go, but f(g(), x) is
		// not, so the sole multi-value argument must be rejected instead of
		// rewriting the call into invalid code. The detection resolves the
		// argument's type, so one-result expressions in any shape (a plain
		// call, a conversion, a parenthesized call) stay allowed. When no type
		// information is available, the check fails open to the historical
		// behavior: synthesized arguments (e.g. a value injected by an earlier
		// aspect) map to no source node, so they reach this branch with no type.
		if len(call.Args) == 1 {
			if tuple, ok := ctx.ResolveType(call.Args[0]).(*types.Tuple); ok && tuple.Len() > 1 {
				return false, fmt.Errorf(
					"append-arguments: cannot append arguments to the call to %s: its sole argument is an expression with %d values",
					callName(call.Fun), tuple.Len(),
				)
			}
		}
		call.Args = append(call.Args, newArgs...)
		return true, nil
	}

	// The call spreads a slice into the variadic parameter, f(a, xs...). Replace
	// the spread argument with one qualified expression:
	//   slices.Concat[[]T](slices.Clone[[]T](xs), []T{injected...})
	//   - Concat always returns a new slice, so the injected arguments are never
	//     written into the spare capacity of the caller's backing array
	//     (DataDog/dd-trace-go#5486);
	//   - the clone completes before any injected call or receive runs: the Go
	//     specification orders calls and receives left to right, but not plain
	//     variable reads, so injected calls cannot change the elements the
	//     callee receives. A plain variable read in a template is unordered
	//     against a side-effecting call in xs, exactly as it is in the non-spread
	//     path;
	//   - xs is evaluated exactly once;
	//   - the generated code declares no identifier and calls no predeclared
	//     function, so it can neither capture a name the templates use nor be
	//     broken by a caller-scope shadow. Its only references are the slices
	//     package and the configured type, and import management chooses their
	//     names.
	lastIdx := len(call.Args) - 1
	arrayType := func() *dst.ArrayType { return &dst.ArrayType{Elt: a.TypeName.AsNode()} }
	qualified := func(name string) *dst.Ident { return &dst.Ident{Name: name, Path: "slices"} }
	call.Args[lastIdx] = &dst.CallExpr{
		Fun: &dst.IndexExpr{
			X:     qualified("Concat"),
			Index: arrayType(),
		},
		Args: []dst.Expr{
			&dst.CallExpr{
				Fun: &dst.IndexExpr{
					X:     qualified("Clone"),
					Index: arrayType(),
				},
				Args: []dst.Expr{call.Args[lastIdx]},
			},
			&dst.CompositeLit{Type: arrayType(), Elts: newArgs},
		},
	}
	// Explicit instantiation needs the go1.18 language and nothing newer:
	// requesting go1.22 would silently switch go1.21 modules to per-iteration
	// loop variables. The slices package itself only needs the toolchain's
	// standard library, not the module's language version.
	ctx.EnsureMinGoLang(context.MustParseGoLangVersion("go1.18"))
	ctx.AddImport("slices", "slices")

	if importPath := a.TypeName.ImportPath; importPath != "" {
		ctx.AddImport(importPath, inferPkgName(importPath))
	}

	return true, nil
}

// callName renders the callee of a call so advice errors can identify the
// call. dst nodes carry no source position, so the callee expression is the
// identification available to advice errors (see #948 for the line mapping).
func callName(expr dst.Expr) string {
	switch expr := expr.(type) {
	case *dst.Ident:
		if expr.Path != "" {
			return fmt.Sprintf("%s.%s", inferPkgName(expr.Path), expr.Name)
		}
		return expr.Name
	case *dst.SelectorExpr:
		return fmt.Sprintf("%s.%s", callName(expr.X), expr.Sel.Name)
	default:
		return "<call>"
	}
}

func (a *appendArgs) AddedImports() []string {
	if len(a.Templates) == 0 {
		// A zero-template advice changes nothing, so it reports no imports at all,
		// even when the configured type comes from another package.
		return nil
	}
	imports := make([]string, 0, len(a.Templates)+2)
	// slices is used when the call spreads a slice into the variadic parameter.
	// The metadata cannot know the call form, so it is reported for any
	// non-empty advice.
	imports = append(imports, "slices")
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

		tn, err := typed.NewTypeName(args.TypeName)
		if err != nil {
			return nil, err
		}

		return AppendArgs(tn, args.Values...), nil
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
