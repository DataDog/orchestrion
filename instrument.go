package orchestrion

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
	"strings"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
	"github.com/dave/dst/decorator/resolver/goast"
	"github.com/dave/dst/decorator/resolver/guess"
)

type ProcessFunc func(string, io.Reader, Config) (io.Reader, error)

type OutputFunc func(string, io.Reader)

func ProcessPackage(name string, process ProcessFunc, output OutputFunc, conf Config) error {
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
		file.Close()
		if err != nil {
			return fmt.Errorf("error scanning file %s: %w", path, err)
		}
		if out != nil {
			output(fullFileName, out)
		}
		return nil
	})
}

func InstrumentFile(name string, content io.Reader, conf Config) (io.Reader, error) {
	fset := token.NewFileSet()
	astFile, err := parser.ParseFile(fset, name, content, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("error parsing content in %s: %w", name, err)
	}

	dec := decorator.NewDecoratorWithImports(fset, name, goast.New())
	f, err := dec.DecorateFile(astFile)
	if err != nil {
		return nil, fmt.Errorf("error decorating file %s: %w", name, err)
	}

	// Use the type checker to extract variable types
	tc := newTypeChecker(dec)
	tc.check(name, fset, astFile)
	for _, decl := range f.Decls {
		if decl, ok := decl.(*dst.FuncDecl); ok {
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

	res := decorator.NewRestorerWithImports(name, guess.New())
	var out bytes.Buffer
	err = res.Fprint(&out, f)
	return &out, err
}

func addSpanCodeToFunction(comment string, decl *dst.FuncDecl, tc *typeChecker) *dst.FuncDecl {
	//check if magic comment is attached to first line
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
		if tc.ofType(firstField.Type, "context.Context") {
			ci = contextInfo{contextType: ident, name: firstField.Names[0].Name, path: firstField.Names[0].Path}
		} else {
			// if not, see if there's an *http.Request parameter. If so, use r.Context()
			for _, v := range decl.Type.Params.List {
				if tc.ofType(v.Type, "*net/http.Request") {
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
	newLines := buildSpanInstrumentation(ci,
		parts,
		funcName)
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
	if contextExpr.contextType != ident {
		return nil
	}

	newLines := []dst.Stmt{
		&dst.AssignStmt{
			Lhs: []dst.Expr{&dst.Ident{Name: contextExpr.name}},
			Tok: token.ASSIGN,
			Rhs: []dst.Expr{
				&dst.CallExpr{
					Fun:  &dst.Ident{Name: "Report", Path: "github.com/datadog/orchestrion"},
					Args: buildArgs(contextExpr, EventStart, name, parts),
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
				Fun:  &dst.Ident{Name: "Report", Path: "github.com/datadog/orchestrion"},
				Args: buildArgs(contextExpr, EventEnd, name, parts),
			},
			Decs: dst.DeferStmtDecorations{NodeDecs: dst.NodeDecs{
				After: dst.NewLine,
				End:   dst.Decorations{"\n", dd_endinstrument},
			}},
		},
	}
	return newLines
}

func buildArgs(contextExpr contextInfo, event Event, name string, parts []string) []dst.Expr {
	out := make([]dst.Expr, 0, len(parts)*2+4)
	out = append(out,
		dupCtxExprForSpan(contextExpr),
		&dst.Ident{Name: event.String(), Path: "github.com/datadog/orchestrion"},
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
		key, val, _ := strings.Cut(v, ":")
		out = append(out, &dst.BasicLit{Kind: token.STRING, Value: `"` + key + `"`})
		out = append(out, &dst.BasicLit{Kind: token.STRING, Value: `"` + val + `"`})
	}
	return out
}

func addInFunctionCode(list []dst.Stmt, tc *typeChecker, conf Config) []dst.Stmt {
	skip := func(stmt dst.Stmt) bool {
		return hasLabel(dd_instrumented, stmt.Decorations().Start.All()) || hasLabel(dd_startinstrument, stmt.Decorations().Start.All()) || hasLabel(dd_startwrap, stmt.Decorations().Start.All())
	}
	out := make([]dst.Stmt, 0, len(list))
	for _, stmt := range list {
		appendStmt := true
		switch stmt := stmt.(type) {
		case *dst.AssignStmt:
			// what we actually care about
			// see if it already has a dd:instrumented
			if skip(stmt) {
				break
			}
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
			wrapSqlOpenFromAssign(stmt, tc)
		case *dst.ExprStmt:
			if skip(stmt) {
				break
			}
			switch conf.HTTPMode {
			case "wrap":
				wrapHandlerFromExpr(stmt, tc)
			case "report":
				reportHandlerFromExpr(stmt, tc, conf)
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
			stmt = instrumentReturn(stmt, tc)
			// 		case *dst.FuncDecl:
			// 			stmt.Body.List = addInFunctionCode(stmt.Body.List, tc)
		}
		if appendStmt {
			out = append(out, stmt)
		}
	}
	return out
}

func instrumentReturn(stmt *dst.ReturnStmt, tc *typeChecker) *dst.ReturnStmt {
	return instrumentSqlReturnCall(stmt, tc)
}

func buildFunctionLiteralHandlerCode(name dst.Expr, funLit *dst.FuncLit) []dst.Stmt {
	//check if magic comment is attached to first line
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

func analyzeExpressionForHandlerLiteral(funLit *dst.FuncLit, tc *typeChecker) bool {
	// check the parameters, see if they match
	inputParams := funLit.Type.Params.List
	return len(inputParams) == 2 &&
		tc.ofType(inputParams[0].Type, "net/http.ResponseWriter") &&
		tc.ofType(inputParams[1].Type, "*net/http.Request")
}

const (
	dd_startinstrument = "//dd:startinstrument"
	dd_endinstrument   = "//dd:endinstrument"
	dd_startwrap       = "//dd:startwrap"
	dd_endwrap         = "//dd:endwrap"
	dd_instrumented    = "//dd:instrumented"
	dd_span            = "//dd:span"
)

func hasLabel(label string, decs []string) bool {
	for _, v := range decs {
		if strings.HasPrefix(v, label) {
			return true
		}
	}
	return false
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

func wrapFromAssign(stmt *dst.AssignStmt, tc *typeChecker) bool {
	return wrapHandlerFromAssign(stmt, tc) ||
		wrapClientFromAssign(stmt, tc)

}

func wrapHandlerFromAssign(stmt *dst.AssignStmt, tc *typeChecker) bool {
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
	if uexpr, ok := stmt.Rhs[0].(*dst.UnaryExpr); ok {
		if x, ok := uexpr.X.(*dst.CompositeLit); ok {
			t, ok := x.Type.(*dst.Ident)
			if !(ok && t.Path == "net/http" && t.Name == "Server") {
				return false
			}
			for _, e := range x.Elts {
				if hasLabel(dd_startwrap, e.Decorations().Start.All()) {
					return false
				}
				if kve, ok := e.(*dst.KeyValueExpr); ok {
					k, ok := kve.Key.(*dst.Ident)
					if !(ok && k.Name == "Handler" && tc.ofType(k, "net/http.Handler")) {
						continue
					}
					kve.Decorations().Start.Append(dd_startwrap)
					kve.Decorations().End.Append("\n", dd_endwrap)
					kve.Value = &dst.CallExpr{
						Fun:  &dst.Ident{Name: "WrapHandler", Path: "github.com/datadog/orchestrion"},
						Args: []dst.Expr{kve.Value},
					}
					return true
				}
			}
		}
	}
	return false
}

func wrapClientFromAssign(stmt *dst.AssignStmt, tc *typeChecker) bool {
	/*
		//dd:startwrap
		c = orchestrion.WrapHTTPClient(client)
		//dd:endwrap
	*/
	if !(len(stmt.Lhs) == 1 && len(stmt.Rhs) == 1) {
		return false
	}
	iden, ok := stmt.Lhs[0].(*dst.Ident)
	if !(ok && tc.ofType(iden, "*net/http.Client")) {
		return false
	}
	stmt.Decorations().Start.Append(dd_startwrap)
	stmt.Decorations().End.Append("\n", dd_endwrap)
	stmt.Rhs[0] = &dst.CallExpr{
		Fun:  &dst.Ident{Name: "WrapHTTPClient", Path: "github.com/datadog/orchestrion"},
		Args: []dst.Expr{stmt.Rhs[0]},
	}
	return true
}

func instrumentSqlReturnCall(stmt *dst.ReturnStmt, tc *typeChecker) *dst.ReturnStmt {
	/*
		//dd:startwrap
		return sql.Open("postgres", "somepath")
		//dd:endwrap

		//dd:startwrap
		return sql.OpenDB(connector)
		//dd:endwrap
	*/
	for _, expr := range stmt.Results {
		fun, ok := expr.(*dst.CallExpr)
		if !ok {
			continue
		}
		if wrapSqlCall(fun) {
			stmt.Decorations().Start.Append(dd_startwrap)
			stmt.Decorations().Before = dst.NewLine
			stmt.Decorations().End.Append("\n", dd_endwrap)
		}

	}
	return stmt
}

func wrapSqlOpenFromAssign(stmt *dst.AssignStmt, tc *typeChecker) bool {
	/*
		//dd:startwrap
		db, err = sql.Open("postgres", "somepath")
		//dd:endwrap

		//dd:startwrap
		db = sql.OpenDB(connector)
		//dd:endwrap
	*/

	rhs := stmt.Rhs[0]
	fun, ok := rhs.(*dst.CallExpr)
	if !ok {
		return false
	}
	if wrapSqlCall(fun) {
		stmt.Decorations().Start.Append(dd_startwrap)
		stmt.Decorations().End.Append("\n", dd_endwrap)
		return true
	}
	return false
}

func wrapSqlCall(call *dst.CallExpr) bool {
	f, ok := call.Fun.(*dst.Ident)
	if !(ok && f.Path == "database/sql" && (f.Name == "Open" || f.Name == "OpenDB")) {
		return false
	}
	f.Path = "github.com/datadog/orchestrion/sql"
	return true
}

func wrapHandlerFromExpr(stmt *dst.ExprStmt, tc *typeChecker) bool {
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
		fun.Decorations().Start.Append(dd_startwrap)
		fun.Decorations().End.Append("\n", dd_endwrap)
		fun.Args[1] = &dst.CallExpr{
			Fun:  &dst.Ident{Name: wrapper, Path: "github.com/datadog/orchestrion"},
			Args: []dst.Expr{fun.Args[1]},
		}
		return true
	}
	if fun, ok := stmt.X.(*dst.CallExpr); ok && len(fun.Args) == 2 {
		switch f := fun.Fun.(type) {
		case *dst.SelectorExpr:
			if tc.ofType(f.X, "*net/http.ServeMux") || tc.ofType(f.X, "net/http.ServeMux") {
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
									Fun: &dst.Ident{Name: "Report", Path: "github.com/datadog/orchestrion"},
									Args: []dst.Expr{
										&dst.CallExpr{Fun: &dst.SelectorExpr{
											X:   &dst.Ident{Name: requestName},
											Sel: &dst.Ident{Name: "Context"},
										}},
										&dst.Ident{Name: "EventCall", Path: "github.com/datadog/orchestrion"},
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
							Fun:  &dst.Ident{Name: "InsertHeader", Path: "github.com/datadog/orchestrion"},
							Args: []dst.Expr{&dst.Ident{Name: requestName}},
						},
					},
				},
				&dst.DeferStmt{Call: &dst.CallExpr{
					Fun: &dst.Ident{Name: "Report", Path: "github.com/datadog/orchestrion"},
					Args: []dst.Expr{
						&dst.CallExpr{Fun: &dst.SelectorExpr{
							X:   &dst.Ident{Name: requestName},
							Sel: &dst.Ident{Name: "Context"},
						}},
						&dst.Ident{Name: "EventReturn", Path: "github.com/datadog/orchestrion"},
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

func addInit(decl *dst.FuncDecl) *dst.FuncDecl {
	//check if magic comment is attached to first line
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
					Fun: &dst.Ident{Path: "github.com/datadog/orchestrion", Name: "Init"},
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

func addCodeToHandler(decl *dst.FuncDecl) *dst.FuncDecl {
	//check if magic comment is attached to first line
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
							Fun: &dst.Ident{Name: "Report", Path: "github.com/datadog/orchestrion"},
							Args: []dst.Expr{
								&dst.CallExpr{Fun: &dst.SelectorExpr{
									X:   &dst.Ident{Name: requestName},
									Sel: &dst.Ident{Name: "Context"},
								}},
								&dst.Ident{Name: "EventStart", Path: "github.com/datadog/orchestrion"},
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
				Fun: &dst.Ident{Name: "Report", Path: "github.com/datadog/orchestrion"},
				Args: []dst.Expr{
					&dst.CallExpr{Fun: &dst.SelectorExpr{
						X:   &dst.Ident{Name: requestName},
						Sel: &dst.Ident{Name: "Context"},
					}},
					&dst.Ident{Name: "EventEnd", Path: "github.com/datadog/orchestrion"},
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

func reportHandlerFromAssign(stmt *dst.AssignStmt, tc *typeChecker, conf Config) {
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
						funLit.Body.List = addInFunctionCode(funLit.Body.List, tc, conf)
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
			funLit.Body.List = addInFunctionCode(funLit.Body.List, tc, conf)
		}
	}
}

func reportHandlerFromExpr(stmt *dst.ExprStmt, tc *typeChecker, conf Config) {
	// might be something we have to recurse on if it's a closure?
	if call, ok := stmt.X.(*dst.CallExpr); ok {
		// check if this is a handler func
		switch funLit := call.Fun.(type) {
		case *dst.FuncLit:
			if analyzeExpressionForHandlerLiteral(funLit, tc) {
				funLit.Body.List = buildFunctionLiteralHandlerCode(nil, funLit)
			}
			funLit.Body.List = addInFunctionCode(funLit.Body.List, tc, conf)
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

func reportHandlerFromDecl(decl *dst.FuncDecl, tc *typeChecker, conf Config) *dst.FuncDecl {
	if conf.HTTPMode != "report" {
		return decl
	}
	// check the parameters, see if they match
	inputParams := decl.Type.Params.List
	if len(inputParams) == 2 &&
		tc.ofType(inputParams[0].Type, "net/http.ResponseWriter") &&
		tc.ofType(inputParams[1].Type, "*net/http.Request") {
		decl = addCodeToHandler(decl)
	}
	return decl
}
