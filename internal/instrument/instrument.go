// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package instrument

import (
	"bytes"
	"fmt"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/datadog/orchestrion/instrument/event"
	"github.com/datadog/orchestrion/internal/config"
	"github.com/datadog/orchestrion/internal/typechecker"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
	"github.com/dave/dst/decorator/resolver/goast"
	"github.com/dave/dst/decorator/resolver/guess"
)

type ProcessFunc func(string, io.Reader, config.Config) (io.Reader, error)

type OutputFunc func(string, io.Reader)

func ProcessPackage(name string, process ProcessFunc, output OutputFunc, conf config.Config) error {
	fileSystem := os.DirFS(name)
	return fs.WalkDir(fileSystem, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("couldn't walk path: %w", err)
		}
		if d.IsDir() || filepath.Ext(path) != ".go" {
			return nil
		}
		fullFileName := name + string(os.PathSeparator) + path
		file, err := os.Open(fullFileName)
		if err != nil {
			return fmt.Errorf("error opening file: %w", err)
		}
		out, err := process(path, file, conf)
		_ = file.Close()
		if err != nil {
			return fmt.Errorf("error scanning file %s: %w", path, err)
		}
		if out != nil {
			output(fullFileName, out)
		}
		return nil
	})
}

func InstrumentFile(name string, content io.Reader, conf config.Config) (io.Reader, error) {
	fset := token.NewFileSet()
	astFile, err := parser.ParseFile(fset, name, content, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("error parsing content in %s: %w", name, err)
	}

	resolver := newResolver()
	dec := decorator.NewDecoratorWithImports(fset, name, goast.WithResolver(resolver))
	f, err := dec.DecorateFile(astFile)
	if err != nil {
		return nil, fmt.Errorf("error decorating file %s: %w", name, err)
	}

	// Use the type checker to extract variable types
	tc := typechecker.New(dec)
	tc.Check(name, fset, astFile)
	for _, decl := range f.Decls {
		if decl, ok := decl.(*dst.FuncDecl); ok {
			decos := decl.Decorations().Start.All()
			if hasLabel(dd_ignore, decos) {
				continue
			}
			// report handlers in top-level functions (functions and methods!)
			decl = reportHandlerFromDecl(decl, tc, conf)
			// find magic comments on functions
			for _, v := range decl.Decorations().Start.All() {
				if strings.HasPrefix(v, dd_span) {
					decl = addSpanCodeToFunction(v, decl, tc)
					break
				}
			}
			// add init to main
			if decl.Name.Name == "main" {
				decl = addInit(decl)
			}
			// wrap or report clients and handlers
			decl.Body.List = addInFunctionCode(decl.Body.List, tc, conf)
		}
	}

	res := decorator.NewRestorerWithImports(name, resolver)
	var out bytes.Buffer
	err = res.Fprint(&out, f)
	return &out, err
}

var (
	specialPackages = map[string]string{
		"github.com/labstack/echo/v4": "echo",
		"github.com/go-chi/chi/v5":    "chi",
	}
)

func newResolver() guess.RestorerResolver {
	// Related to dave/dst#44, the default guess resolver used by goast assumes
	// the last segment of the import path is the package name.
	// This behavior leads to unexpected package names in cases like github.com/labstack/echo/v4,
	// as guess assumes the package name to be v4 instead of echo.
	// We could use gopackages, but it's slower. Benchmark anecdata: guess <0s; gopackages ~1.5s.
	r := guess.WithMap(specialPackages)
	return r
}

func addSpanCodeToFunction(comment string, decl *dst.FuncDecl, tc *typechecker.TypeChecker) *dst.FuncDecl {
	// check if magic comment is attached to first line
	if len(decl.Body.List) > 0 {
		decs := decl.Body.List[0].Decorations().Start
		for _, v := range decs.All() {
			if strings.HasPrefix(v, dd_startinstrument) {
				log.Println("already instrumented")
				return decl
			}
		}
	}

	start := len(dd_span)
	// get the tags from the magic comment
	parts := strings.Split(comment[start:], " ")
	if parts[0] == "" {
		parts = parts[1:]
	}

	// get function name
	funcName := decl.Name.String()
	// get context parameter
	var ci contextInfo
	if len(decl.Type.Params.List) > 0 {
		// first see if the 1st parameter of the function is a context. If so, use it
		firstField := decl.Type.Params.List[0]
		if tc.OfType(firstField.Type, "context.Context") {
			ci = contextInfo{contextType: ident, name: firstField.Names[0].Name, path: firstField.Names[0].Path}
		} else {
			// if not, see if there's an *http.Request parameter. If so, use r.Context()
			for _, v := range decl.Type.Params.List {
				if tc.OfType(v.Type, "*net/http.Request") {
					ci = contextInfo{contextType: call, name: v.Names[0].Name, path: v.Names[0].Path}
					break
				}
			}
		}
	}
	// if no context, cannot use the span comment
	if ci.contextType == 0 {
		log.Println("no context in function parameters, cannot instrument", funcName)
		return decl
	}
	newLines := buildSpanInstrumentation(ci, parts, funcName)
	decl.Body.List = append(newLines, decl.Body.List...)
	return decl
}

type contextType int

const (
	_ contextType = iota
	ident
	call
)

type contextInfo struct {
	contextType contextType
	name        string
	path        string
}

func buildSpanInstrumentation(contextExpr contextInfo, parts []string, name string) []dst.Stmt {
	/*
		lines to insert:
			//dd:startinstrument
			contextIdent = Report(contextIdent, EventStart, "name", "doThing", parts...)
			defer Report(contextIdent, EventEnd, "name", "doThing", parts...)
			//dd:endinstrument
	*/
	return []dst.Stmt{
		buildReportStmt(contextExpr, parts, name),
		buildReportDeferStmt(contextExpr, parts, name),
	}
}

func buildReportStmt(contextExpr contextInfo, parts []string, name string) dst.Stmt {
	var rhs []dst.Expr
	switch contextExpr.contextType {
	case ident:
		rhs = []dst.Expr{buildReportCallExpr(contextExpr, parts, name)}
	case call:
		rhs = []dst.Expr{
			&dst.CallExpr{
				Fun: &dst.SelectorExpr{
					X:   &dst.Ident{Name: contextExpr.name, Path: contextExpr.path},
					Sel: &dst.Ident{Name: "WithContext"},
				},
				Args: []dst.Expr{buildReportCallExpr(contextExpr, parts, name)},
			},
		}
	}
	return &dst.AssignStmt{
		Lhs: []dst.Expr{&dst.Ident{Name: contextExpr.name}},
		Tok: token.ASSIGN,
		Rhs: rhs,
		Decs: dst.AssignStmtDecorations{NodeDecs: dst.NodeDecs{
			Before: dst.NewLine,
			Start:  dst.Decorations{dd_startinstrument},
			After:  dst.NewLine,
		}},
	}
}

func buildReportCallExpr(contextExpr contextInfo, parts []string, name string) dst.Expr {
	return &dst.CallExpr{
		Fun:  &dst.Ident{Name: "Report", Path: "github.com/datadog/orchestrion/instrument"},
		Args: buildArgs(contextExpr, event.EventStart, name, parts),
	}
}

func buildReportDeferStmt(contextExpr contextInfo, parts []string, name string) *dst.DeferStmt {
	return &dst.DeferStmt{
		Call: &dst.CallExpr{
			Fun:  &dst.Ident{Name: "Report", Path: "github.com/datadog/orchestrion/instrument"},
			Args: buildArgs(contextExpr, event.EventEnd, name, parts),
		},
		Decs: dst.DeferStmtDecorations{NodeDecs: dst.NodeDecs{
			After: dst.NewLine,
			End:   dst.Decorations{"\n", dd_endinstrument},
		}},
	}
}

func buildArgs(contextExpr contextInfo, event event.Event, name string, parts []string) []dst.Expr {
	out := make([]dst.Expr, 0, len(parts)*2+4)
	out = append(out,
		dupCtxExprForSpan(contextExpr),
		&dst.Ident{Name: event.String(), Path: "github.com/datadog/orchestrion/instrument/event"},
		&dst.BasicLit{Kind: token.STRING, Value: `"function-name"`},
		&dst.BasicLit{Kind: token.STRING, Value: fmt.Sprintf(`"%s"`, name)},
	)
	out = append(out, buildExprsFromParts(parts)...)

	return out
}

func dupCtxExprForSpan(in contextInfo) dst.Expr {
	// only expecting r.Context() or ctx
	switch in.contextType {
	case call:
		return &dst.CallExpr{
			Fun: &dst.SelectorExpr{
				X:   &dst.Ident{Name: in.name, Path: in.path},
				Sel: &dst.Ident{Name: "Context"},
			},
		}
	case ident:
		return &dst.Ident{Name: in.name, Path: in.path}
	}
	panic(fmt.Sprintf("unexpected contextInfo %#v", in))
}

func buildExprsFromParts(parts []string) []dst.Expr {
	out := make([]dst.Expr, 0, len(parts)*2)
	for _, v := range parts {
		key, val, found := strings.Cut(v, ":")
		out = append(out, &dst.BasicLit{Kind: token.STRING, Value: `"` + key + `"`})
		if found {
			out = append(out, &dst.BasicLit{Kind: token.STRING, Value: `"` + val + `"`})
		} else {
			out = append(out, &dst.BasicLit{Kind: token.VAR, Value: key})
		}
	}
	return out
}

func skipInstrumentation(stmt dst.Stmt) bool {
	decos := stmt.Decorations().Start.All()
	return hasLabel(dd_instrumented, decos) ||
		hasLabel(dd_startinstrument, decos) ||
		hasLabel(dd_startwrap, decos) ||
		hasLabel(dd_ignore, decos)
}

func addInFunctionCode(list []dst.Stmt, tc *typechecker.TypeChecker, conf config.Config) []dst.Stmt {
	out := make([]dst.Stmt, 0, len(list))
	for _, stmt := range list {
		if skipInstrumentation(stmt) {
			out = append(out, stmt)
			continue
		}
		appendStmt := true
		switch stmt := stmt.(type) {
		case *dst.AssignStmt:
			switch conf.HTTPMode {
			case "wrap":
				wrapFromAssign(stmt, tc)
			case "report":
				if requestName, ok := analyzeStmtForRequestClient(stmt); ok {
					stmt.Decorations().Start.Prepend(dd_instrumented)
					out = append(out, stmt)
					appendStmt = false
					out = append(out, buildRequestClientCode(requestName))
				}
				reportHandlerFromAssign(stmt, tc, conf)
			}
			wrapSqlOpenFromAssign(stmt)
			wrapGRPC(stmt)
			wrapGorillaMux(stmt)
			if r := instrumentGin(stmt); r != nil {
				appendStmt = false
				out = append(out, r...)
			}
			if r := instrumentEchoV4(stmt); r != nil {
				appendStmt = false
				out = append(out, r...)
			}
			if r := instrumentChiV5(stmt); r != nil {
				appendStmt = false
				out = append(out, r...)
			}

			// Recurse when there is a function literal on the RHS of the assignment.
			for _, expr := range stmt.Rhs {
				if compLit, ok := expr.(*dst.CompositeLit); ok {
					for _, v := range compLit.Elts {
						if kv, ok := v.(*dst.KeyValueExpr); ok {
							if funLit, ok := kv.Value.(*dst.FuncLit); ok {
								funLit.Body.List = addInFunctionCode(funLit.Body.List, tc, conf)
							}
						}
					}
				}
				if funLit, ok := expr.(*dst.FuncLit); ok {
					funLit.Body.List = addInFunctionCode(funLit.Body.List, tc, conf)
				}
			}
		case *dst.ExprStmt:
			switch conf.HTTPMode {
			case "wrap":
				wrapHandlerFromExpr(stmt, tc)
			case "report":
				reportHandlerFromExpr(stmt, tc, conf)
			}
			if call, ok := stmt.X.(*dst.CallExpr); ok {
				switch funLit := call.Fun.(type) {
				case *dst.FuncLit:
					funLit.Body.List = addInFunctionCode(funLit.Body.List, tc, conf)
				}
			}
		case *dst.GoStmt:
			if conf.HTTPMode == "report" {
				if funLit, ok := stmt.Call.Fun.(*dst.FuncLit); ok {
					// check for function literal that is a handler
					if analyzeExpressionForHandlerLiteral(funLit, tc) {
						funLit.Body.List = buildFunctionLiteralHandlerCode(nil, funLit)
					}
					funLit.Body.List = addInFunctionCode(funLit.Body.List, tc, conf)
				}
			}
		case *dst.DeferStmt:
			if conf.HTTPMode == "report" {
				if funLit, ok := stmt.Call.Fun.(*dst.FuncLit); ok {
					// check for function literal that is a handler
					if analyzeExpressionForHandlerLiteral(funLit, tc) {
						funLit.Body.List = buildFunctionLiteralHandlerCode(nil, funLit)
					}
					funLit.Body.List = addInFunctionCode(funLit.Body.List, tc, conf)
				}
			}
		case *dst.BlockStmt:
			stmt.List = addInFunctionCode(stmt.List, tc, conf)
		case *dst.CaseClause:
			stmt.Body = addInFunctionCode(stmt.Body, tc, conf)
		case *dst.CommClause:
			stmt.Body = addInFunctionCode(stmt.Body, tc, conf)
		case *dst.IfStmt:
			stmt.Body.List = addInFunctionCode(stmt.Body.List, tc, conf)
		case *dst.SwitchStmt:
			stmt.Body.List = addInFunctionCode(stmt.Body.List, tc, conf)
		case *dst.TypeSwitchStmt:
			stmt.Body.List = addInFunctionCode(stmt.Body.List, tc, conf)
		case *dst.SelectStmt:
			stmt.Body.List = addInFunctionCode(stmt.Body.List, tc, conf)
		case *dst.ForStmt:
			stmt.Body.List = addInFunctionCode(stmt.Body.List, tc, conf)
		case *dst.RangeStmt:
			stmt.Body.List = addInFunctionCode(stmt.Body.List, tc, conf)
		case *dst.ReturnStmt:
			stmt = instrumentReturn(stmt)
		}
		if appendStmt {
			out = append(out, stmt)
		}
	}
	return out
}

func instrumentReturn(stmt *dst.ReturnStmt) *dst.ReturnStmt {
	return wrapSqlReturnCall(stmt)
}

const (
	dd_startinstrument = "//dd:startinstrument"
	dd_endinstrument   = "//dd:endinstrument"
	dd_startwrap       = "//dd:startwrap"
	dd_endwrap         = "//dd:endwrap"
	dd_instrumented    = "//dd:instrumented"
	dd_span            = "//dd:span"
	dd_ignore          = "//dd:ignore"
)

func hasLabel(label string, decs []string) bool {
	for _, v := range decs {
		if strings.HasPrefix(v, label) {
			return true
		}
	}
	return false
}

func addInit(decl *dst.FuncDecl) *dst.FuncDecl {
	// check if magic comment is attached to first line
	if len(decl.Body.List) > 0 {
		decs := decl.Body.List[0].Decorations().Start
		for _, v := range decs.All() {
			if strings.HasPrefix(v, dd_startinstrument) {
				log.Println("already instrumented")
				return decl
			}
		}
	}

	newLines := []dst.Stmt{
		&dst.DeferStmt{
			Call: &dst.CallExpr{
				Fun: &dst.CallExpr{
					Fun: &dst.Ident{Path: "github.com/datadog/orchestrion/instrument", Name: "Init"},
				},
			},
			Decs: dst.DeferStmtDecorations{NodeDecs: dst.NodeDecs{
				Start: dst.Decorations{"\n", dd_startinstrument},
				End:   dst.Decorations{"\n", dd_endinstrument},
			}},
		},
	}

	decl.Body.List = append(newLines, decl.Body.List...)
	return decl
}

func buildFunctionInstrumentation(funcName dst.Expr, requestName string) []dst.Stmt {
	/*
		lines to insert:
			//dd:startinstrument
			r = r.WithContext(Report(r.Context(), EventStart, "name", "doThing", "verb", r.Method))
			defer Report(r.Context(), EventEnd, "name", "doThing", "verb", r.Method)
			//dd:endinstrument
	*/
	if funcName == nil {
		funcName = &dst.BasicLit{Kind: token.STRING, Value: `"anon"`}
	}
	newLines := []dst.Stmt{
		&dst.AssignStmt{
			Lhs: []dst.Expr{&dst.Ident{Name: requestName}},
			Tok: token.ASSIGN,
			Rhs: []dst.Expr{
				&dst.CallExpr{
					Fun: &dst.SelectorExpr{
						X:   &dst.Ident{Name: requestName},
						Sel: &dst.Ident{Name: "WithContext"},
					},
					Args: []dst.Expr{
						&dst.CallExpr{
							Fun: &dst.Ident{Name: "Report", Path: "github.com/datadog/orchestrion/instrument"},
							Args: []dst.Expr{
								&dst.CallExpr{Fun: &dst.SelectorExpr{
									X:   &dst.Ident{Name: requestName},
									Sel: &dst.Ident{Name: "Context"},
								}},
								&dst.Ident{Name: "EventStart", Path: "github.com/datadog/orchestrion/instrument/event"},
								&dst.BasicLit{Kind: token.STRING, Value: `"name"`},
								dup(funcName),
								&dst.BasicLit{Kind: token.STRING, Value: `"verb"`},
								&dst.SelectorExpr{
									X:   &dst.Ident{Name: requestName},
									Sel: &dst.Ident{Name: "Method"},
								},
							},
						},
					},
				},
			},
			Decs: dst.AssignStmtDecorations{NodeDecs: dst.NodeDecs{
				Before: dst.NewLine,
				Start:  dst.Decorations{dd_startinstrument},
				After:  dst.NewLine,
			}},
		},
		&dst.DeferStmt{
			Call: &dst.CallExpr{
				Fun: &dst.Ident{Name: "Report", Path: "github.com/datadog/orchestrion/instrument"},
				Args: []dst.Expr{
					&dst.CallExpr{Fun: &dst.SelectorExpr{
						X:   &dst.Ident{Name: requestName},
						Sel: &dst.Ident{Name: "Context"},
					}},
					&dst.Ident{Name: "EventEnd", Path: "github.com/datadog/orchestrion/instrument/event"},
					&dst.BasicLit{Kind: token.STRING, Value: `"name"`},
					dup(funcName),
					&dst.BasicLit{Kind: token.STRING, Value: `"verb"`},
					&dst.SelectorExpr{
						X:   &dst.Ident{Name: requestName},
						Sel: &dst.Ident{Name: "Method"},
					},
				},
			},
			Decs: dst.DeferStmtDecorations{NodeDecs: dst.NodeDecs{
				After: dst.NewLine,
				End:   dst.Decorations{"\n", dd_endinstrument},
			}},
		},
	}
	return newLines
}

func dup(in dst.Expr) dst.Expr {
	switch in := in.(type) {
	case *dst.Ident:
		return &dst.BasicLit{Kind: token.STRING, Value: fmt.Sprintf(`"%s"`, in.Name)}
	case *dst.SelectorExpr:
		return &dst.SelectorExpr{
			X:   dup(in.X),
			Sel: dup(in.Sel).(*dst.Ident),
		}
	case *dst.IndexExpr:
		return &dst.IndexExpr{
			X:     dup(in.X),
			Index: dup(in.Index),
		}
	case *dst.BasicLit:
		return &dst.BasicLit{Kind: in.Kind, Value: in.Value}
	default:
		return &dst.BasicLit{Kind: token.STRING, Value: "unknown"}
	}
}

func funcIdent(e dst.Expr) (*dst.Ident, bool) {
	call, ok := e.(*dst.CallExpr)
	if !ok {
		return nil, false
	}
	f, ok := call.Fun.(*dst.Ident)
	if !ok {
		return nil, false
	}
	return f, true
}

// useMiddleware returns a statement that uses the given middleware.
func useMiddleware(expr dst.Expr, middleware string) (*dst.ExprStmt, error) {
	/*
		//dd:instrumented
		r := echo.New()
		//dd:startinstrument
		r.Use(instrument.EchoV4Middleware())
		//dd:endinstrument

		//dd:instrumented
		api.server = echo.New()
		//dd:startinstrument
		api.serverr.Use(instrument.EchoV4Middleware())
		//dd:endinstrument
	*/
	var stmt *dst.ExprStmt
	switch ex := expr.(type) {
	case *dst.Ident:
		stmt = &dst.ExprStmt{
			X: &dst.CallExpr{
				Fun: &dst.SelectorExpr{
					X:   &dst.Ident{Name: ex.Name},
					Sel: &dst.Ident{Name: "Use"},
				},
				Args: []dst.Expr{
					&dst.CallExpr{
						Fun: &dst.Ident{
							Name: middleware,
							Path: "github.com/datadog/orchestrion/instrument",
						},
					},
				},
			},
		}
	case *dst.SelectorExpr:
		x, ok := ex.X.(*dst.Ident)
		if !ok {
			return nil, fmt.Errorf("unexpected type %v", reflect.TypeOf(ex.X))
		}
		stmt = &dst.ExprStmt{
			X: &dst.CallExpr{
				Fun: &dst.SelectorExpr{
					X: &dst.SelectorExpr{
						X:   &dst.Ident{Name: x.Name},
						Sel: &dst.Ident{Name: ex.Sel.Name},
					},
					Sel: &dst.Ident{Name: "Use"},
				},
				Args: []dst.Expr{
					&dst.CallExpr{
						Fun: &dst.Ident{
							Name: middleware,
							Path: "github.com/datadog/orchestrion/instrument",
						},
					},
				},
			},
		}
	default:
		return nil, fmt.Errorf("unexpected type %v", reflect.TypeOf(expr))
	}
	markAsInstrumented(stmt)
	return stmt, nil
}

func instrumentMiddleware(stmt *dst.AssignStmt, check func(*dst.AssignStmt) bool, middleware string) []dst.Stmt {
	if !check(stmt) {
		return nil
	}
	instrumented, err := useMiddleware(stmt.Lhs[0], middleware)
	if err != nil {
		fmt.Println("error instrumenting middleware", err)
		return nil
	}
	stmt.Decorations().Start.Prepend(dd_instrumented)
	return []dst.Stmt{
		stmt,
		instrumented,
	}
}

func markAsWrap(stmt dst.Node) {
	stmt.Decorations().Start.Append(dd_startwrap)
	stmt.Decorations().End.Append("\n", dd_endwrap)
}

func markAsInstrumented(stmt dst.Node) {
	stmt.Decorations().Start.Append(dd_startinstrument)
	stmt.Decorations().End.Append("\n", dd_endinstrument)
}
