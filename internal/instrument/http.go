// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package instrument

import (
	"go/token"
	"log"
	"strings"

	"github.com/datadog/orchestrion/internal/config"
	"github.com/datadog/orchestrion/internal/typechecker"

	"github.com/dave/dst"
)

func analyzeExpressionForHandlerLiteral(funLit *dst.FuncLit, tc *typechecker.TypeChecker) bool {
	// check the parameters, see if they match
	inputParams := funLit.Type.Params.List
	return len(inputParams) == 2 &&
		tc.OfType(inputParams[0].Type, "net/http.ResponseWriter") &&
		tc.OfType(inputParams[1].Type, "*net/http.Request")
}

func analyzeStmtForRequestClient(stmt *dst.AssignStmt) (string, bool) {
	// looking for
	// 	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "localhost:8080", strings.NewReader(os.Args[1]))
	// has 2 return values (*http.Request and error)
	// function named NewRequestWithContext
	if len(stmt.Lhs) == 2 &&
		len(stmt.Rhs) == 1 {
		if fun, ok := stmt.Rhs[0].(*dst.CallExpr); ok {
			if iden, ok := fun.Fun.(*dst.Ident); ok {
				if iden.Name == "NewRequestWithContext" && iden.Path == "net/http" {
					if iden, ok := stmt.Lhs[0].(*dst.Ident); ok {
						return iden.Name, true
					}
				}
			}
		}
	}
	return "", false
}

func buildFunctionLiteralHandlerCode(name dst.Expr, funLit *dst.FuncLit) []dst.Stmt {
	// check if magic comment is attached to first line
	if len(funLit.Body.List) > 0 {
		decs := funLit.Body.List[0].Decorations().Start
		for _, v := range decs.All() {
			if strings.HasPrefix(v, dd_startinstrument) {
				log.Println("already instrumented")
				return funLit.Body.List
			}
		}
	}
	// get name of request var
	requestName := funLit.Type.Params.List[1].Names[0].Name
	newLines := buildFunctionInstrumentation(name, requestName)
	funLit.Body.List = append(newLines, funLit.Body.List...)
	return funLit.Body.List
}

func wrapFromAssign(stmt *dst.AssignStmt, tc *typechecker.TypeChecker) bool {
	return wrapHandlerFromAssign(stmt, tc) || wrapClientFromAssign(stmt, tc)
}

func wrapHandlerFromAssign(stmt *dst.AssignStmt, tc *typechecker.TypeChecker) bool {
	/*
		s = &http.Server{
			//dd:startwrap
			Handler: orchestrion.WrapHandler(handler),
			//dd:endwrap
		}
	*/
	if !(len(stmt.Lhs) == 1 && len(stmt.Rhs) == 1) {
		return false
	}
	uexpr, ok := stmt.Rhs[0].(*dst.UnaryExpr)
	if !ok {
		return false
	}
	x, ok := uexpr.X.(*dst.CompositeLit)
	if !ok {
		return false
	}
	t, ok := x.Type.(*dst.Ident)
	if !(ok && t.Path == "net/http" && t.Name == "Server") {
		return false
	}
	for _, e := range x.Elts {
		if hasLabel(dd_startwrap, e.Decorations().Start.All()) {
			return false
		}
		kve, ok := e.(*dst.KeyValueExpr)
		if !ok {
			continue
		}
		k, ok := kve.Key.(*dst.Ident)
		if !(ok && k.Name == "Handler" && tc.OfType(k, "net/http.Handler")) {
			continue
		}
		markAsWrap(kve)
		kve.Value = &dst.CallExpr{
			Fun:  &dst.Ident{Name: "WrapHandler", Path: "github.com/datadog/orchestrion/instrument"},
			Args: []dst.Expr{kve.Value},
		}
		return true
	}
	return false
}

func wrapClientFromAssign(stmt *dst.AssignStmt, tc *typechecker.TypeChecker) bool {
	/*
		//dd:startwrap
		c = orchestrion.WrapHTTPClient(client)
		//dd:endwrap
	*/
	if !(len(stmt.Lhs) == 1 && len(stmt.Rhs) == 1) {
		return false
	}
	iden, ok := stmt.Lhs[0].(*dst.Ident)
	if !(ok && tc.OfType(iden, "*net/http.Client")) {
		return false
	}
	markAsWrap(stmt)
	stmt.Rhs[0] = &dst.CallExpr{
		Fun:  &dst.Ident{Name: "WrapHTTPClient", Path: "github.com/datadog/orchestrion/instrument"},
		Args: []dst.Expr{stmt.Rhs[0]},
	}
	return true
}

func wrapHandlerFromExpr(stmt *dst.ExprStmt, tc *typechecker.TypeChecker) bool {
	/*
		//dd:startwrap
		http.Handle("/handle", orchestrion.WrapHandler(handler))
		//dd:endwrap

		//dd:startwrap
		http.HandleFunc("/handle", orchestrion.WrapHandlerFunc(handler))
		//dd:endwrap

		//dd:startwrap
		s.Handle("/handle", orchestrion.WrapHandler(handler))
		//dd:endwrap

		//dd:startwrap
		s.HandleFunc("/handle", orchestrion.WrapHandlerFunc(handler))
		//dd:endwrap
	*/
	wrap := func(fun *dst.CallExpr, name string) bool {
		var wrapper string
		switch name {
		case "Handle":
			wrapper = "WrapHandler"
		case "HandleFunc":
			wrapper = "WrapHandlerFunc"
		default:
			return false
		}
		markAsWrap(fun)
		fun.Args[1] = &dst.CallExpr{
			Fun:  &dst.Ident{Name: wrapper, Path: "github.com/datadog/orchestrion/instrument"},
			Args: []dst.Expr{fun.Args[1]},
		}
		return true
	}
	if fun, ok := stmt.X.(*dst.CallExpr); ok && len(fun.Args) == 2 {
		switch f := fun.Fun.(type) {
		case *dst.SelectorExpr:
			if tc.OfType(f.X, "*net/http.ServeMux") || tc.OfType(f.X, "net/http.ServeMux") {
				return wrap(fun, f.Sel.Name)
			}
		case *dst.Ident:
			if f.Path == "net/http" {
				return wrap(fun, f.Name)
			}
		}
	}
	return false
}

func buildRequestClientCode(requestName string) dst.Stmt {
	/*
		//dd:startinstrument
		if req != nil {
			req = req.WithContext(Report(req.Context(), EventCall, "url", req.URL, "method", req.Method))
			req = InsertHeader(req)
			defer Report(req.Context(), EventReturn, "url", req.URL, "method", req.Method)
		}
		//dd:endinstrument
	*/
	return &dst.IfStmt{
		Cond: &dst.BinaryExpr{
			X:  &dst.Ident{Name: requestName},
			Op: token.NEQ,
			Y:  &dst.Ident{Name: "nil"},
		},
		Body: &dst.BlockStmt{
			List: []dst.Stmt{
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
										&dst.Ident{Name: "EventCall", Path: "github.com/datadog/orchestrion/instrument/event"},
										&dst.BasicLit{Kind: token.STRING, Value: `"name"`},
										&dst.SelectorExpr{
											X:   &dst.Ident{Name: requestName},
											Sel: &dst.Ident{Name: "URL"},
										},
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
				},
				&dst.AssignStmt{
					Lhs: []dst.Expr{&dst.Ident{Name: requestName}},
					Tok: token.ASSIGN,
					Rhs: []dst.Expr{
						&dst.CallExpr{
							Fun:  &dst.Ident{Name: "InsertHeader", Path: "github.com/datadog/orchestrion/instrument"},
							Args: []dst.Expr{&dst.Ident{Name: requestName}},
						},
					},
				},
				&dst.DeferStmt{Call: &dst.CallExpr{
					Fun: &dst.Ident{Name: "Report", Path: "github.com/datadog/orchestrion/instrument"},
					Args: []dst.Expr{
						&dst.CallExpr{Fun: &dst.SelectorExpr{
							X:   &dst.Ident{Name: requestName},
							Sel: &dst.Ident{Name: "Context"},
						}},
						&dst.Ident{Name: "EventReturn", Path: "github.com/datadog/orchestrion/instrument/event"},
						&dst.BasicLit{Kind: token.STRING, Value: `"name"`},
						&dst.SelectorExpr{
							X:   &dst.Ident{Name: requestName},
							Sel: &dst.Ident{Name: "URL"},
						},
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
		Decs: dst.IfStmtDecorations{
			NodeDecs: dst.NodeDecs{
				Before: dst.NewLine,
				Start:  dst.Decorations{dd_startinstrument},
				After:  dst.NewLine,
				End:    dst.Decorations{"\n", dd_endinstrument},
			},
		},
	}
}

func reportHandlerFromDecl(decl *dst.FuncDecl, tc *typechecker.TypeChecker, conf config.Config) *dst.FuncDecl {
	if conf.HTTPMode != "report" {
		return decl
	}
	// check the parameters, see if they match
	inputParams := decl.Type.Params.List
	if len(inputParams) == 2 &&
		tc.OfType(inputParams[0].Type, "net/http.ResponseWriter") &&
		tc.OfType(inputParams[1].Type, "*net/http.Request") {
		decl = addCodeToHandler(decl)
	}
	return decl
}

func addCodeToHandler(decl *dst.FuncDecl) *dst.FuncDecl {
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
	// get name of request var
	requestName := decl.Type.Params.List[1].Names[0].Name
	newLines := buildFunctionInstrumentation(
		&dst.BasicLit{Kind: token.STRING, Value: `"` + decl.Name.Name + `"`},
		requestName)
	decl.Body.List = append(newLines, decl.Body.List...)
	return decl
}

func reportHandlerFromAssign(stmt *dst.AssignStmt, tc *typechecker.TypeChecker, conf config.Config) {
	// check for function literal that is a handler
	for pos, expr := range stmt.Rhs {
		if compLit, ok := expr.(*dst.CompositeLit); ok {
			for _, v := range compLit.Elts {
				if kv, ok := v.(*dst.KeyValueExpr); ok {
					if funLit, ok := kv.Value.(*dst.FuncLit); ok {
						if analyzeExpressionForHandlerLiteral(funLit, tc) {
							// get the name from the field
							funLit.Body.List = buildFunctionLiteralHandlerCode(kv.Key, funLit)
						}
					}
				}
			}
		}
		if funLit, ok := expr.(*dst.FuncLit); ok {
			if analyzeExpressionForHandlerLiteral(funLit, tc) {
				// get the name from the lhs in the same position -- if it's not there, exit, code isn't going to compile
				if len(stmt.Lhs) <= pos {
					break
				}
				funLit.Body.List = buildFunctionLiteralHandlerCode(stmt.Lhs[pos], funLit)
			}
		}
	}
}

func reportHandlerFromExpr(stmt *dst.ExprStmt, tc *typechecker.TypeChecker, conf config.Config) {
	// might be something we have to recurse on if it's a closure?
	if call, ok := stmt.X.(*dst.CallExpr); ok {
		// check if this is a handler func
		switch funLit := call.Fun.(type) {
		case *dst.FuncLit:
			if analyzeExpressionForHandlerLiteral(funLit, tc) {
				funLit.Body.List = buildFunctionLiteralHandlerCode(nil, funLit)
			}
		}
		// check if any of the parameters is a function literal
		var prevExpr dst.Expr
		for _, v := range call.Args {
			if funLit, ok := v.(*dst.FuncLit); ok {
				// check for function literal that is a handler
				if analyzeExpressionForHandlerLiteral(funLit, tc) {
					funLit.Body.List = buildFunctionLiteralHandlerCode(prevExpr, funLit)
				}
			}
			prevExpr = v
		}
	}
}

// unwrapClient unwraps client, to be used in dst.Inspect.
// Returns true to continue the traversal, false to stop.
func unwrapClient(n dst.Node) bool {
	s, ok := n.(*dst.AssignStmt)
	if !ok {
		return true
	}
	ce, ok := s.Rhs[0].(*dst.CallExpr)
	if !ok {
		return true
	}
	cei, ok := ce.Fun.(*dst.Ident)
	if !ok {
		return true
	}
	if cei.Path == "github.com/datadog/orchestrion/instrument" && strings.HasPrefix(cei.Name, "WrapHTTPClient") {
		s.Rhs[0] = ce.Args[0]
		return false
	}
	return true
}

// unwrapHandlerExpr unwraps handler expressions, to be used in dst.Inspect.
// Returns true to continue the traversal, false to stop.
func unwrapHandlerExpr(n dst.Node) bool {
	es, ok := n.(*dst.ExprStmt)
	if !ok {
		return true
	}
	f, ok := es.X.(*dst.CallExpr)
	if !ok {
		return true
	}
	if len(f.Args) <= 1 {
		return true
	}
	cei, ok := funcIdent(f.Args[1])
	if !ok {
		return true
	}
	if !(cei.Path == "github.com/datadog/orchestrion/instrument" &&
		// This catches both WrapHandler *and* WrapHandlerFunc
		strings.HasPrefix(cei.Name, "WrapHandler")) {
		return true
	}
	ce := f.Args[1].(*dst.CallExpr)
	f.Args[1] = ce.Args[0]
	return false
}

// unwrapHandlerAssign unwraps handler assignements, to be used in dst.Inspect.
// Returns true to continue the traversal, false to stop.
func unwrapHandlerAssign(n dst.Node) bool {
	es, ok := n.(*dst.KeyValueExpr)
	if !ok {
		return true
	}
	f, ok := es.Value.(*dst.CallExpr)
	if !ok {
		return true
	}
	if len(f.Args) < 1 {
		return true
	}
	iden, ok := f.Fun.(*dst.Ident)
	if !ok {
		return true
	}
	if !(iden.Path == "github.com/datadog/orchestrion/instrument" &&
		// This catches both WrapHandler *and* WrapHandlerFunc
		strings.HasPrefix(iden.Name, "WrapHandler")) {
		return true
	}
	es.Value = f.Args[0]
	return false
}
