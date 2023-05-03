package orchestrion

import (
	"bytes"
	"fmt"
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

func ScanPackage(name string, process func(string, io.Reader)) error {
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
		out, err := ScanFile(path, file)
		file.Close()
		if err != nil {
			return fmt.Errorf("error scanning file %s: %w", path, err)
		}
		if out != nil {
			process(fullFileName, out)
		}
		return nil
	})
}
func ScanFile(name string, content io.Reader) (io.Reader, error) {
	fset := token.NewFileSet()
	d := decorator.NewDecoratorWithImports(fset, name, goast.New())
	f, err := d.Parse(content)
	if err != nil {
		return nil, fmt.Errorf("error parsing content in %s: %w", name, err)
	}

	// see if this file should be modified (see if it imports net/http)

	// server support stage 1: find handlers in top-level functions (functions and methods!)
	for _, decl := range f.Decls {
		if decl, ok := decl.(*dst.FuncDecl); ok {
			// check the parameters, see if they match
			inputParams := decl.Type.Params.List
			if len(inputParams) == 2 &&
				IsType(inputParams[0].Type, "net/http", "ResponseWriter") &&
				IsPointerType(inputParams[1].Type, "net/http", "Request") {
				decl = addCodeToHandler(decl)
			}
			// support stage 3: find magic comments on functions
			for _, v := range decl.Decorations().Start.All() {
				if strings.HasPrefix(v, "//dd:span") {
					decl = addSpanCodeToFunction(v, decl)
					break
				}
			}
			// scan body for request creation or handlers as function literals
			// client support stage 1: find http clients in functions
			// server support stage 2: find closures in functions to instrument too
			decl.Body.List = addInFunctionCode(decl.Body.List)
		}
	}

	res := decorator.NewRestorerWithImports(name, guess.New())
	var out bytes.Buffer
	err = res.Fprint(&out, f)
	return &out, err
}

func addSpanCodeToFunction(comment string, decl *dst.FuncDecl) *dst.FuncDecl {
	//check if magic comment is attached to first line
	if len(decl.Body.List) > 0 {
		decs := decl.Body.List[0].Decorations().Start
		for _, v := range decs.All() {
			if strings.HasPrefix(v, "//dd:startinstrument") {
				log.Println("already instrumented")
				return decl
			}
		}
	}

	start := len("//dd:span")
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
		if IsType(firstField.Type, "context", "Context") {
			ci = contextInfo{contextType: ident, name: firstField.Names[0].Name, path: firstField.Names[0].Path}
		} else {
			// if not, see if there's an *http.Request parameter. If so, use r.Context()
			for _, v := range decl.Type.Params.List {
				if IsPointerType(v.Type, "net/http", "Request") {
					ci = contextInfo{contextType: call, name: v.Names[0].Name, path: v.Names[0].Path}
					break
				}
			}
		}
	}
	// if no context, cannot use the span comment
	if ci.contextType == 0 {
		log.Println("no context in function parameters, cannot instrument")
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
				Start:  dst.Decorations{"//dd:startinstrument"},
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
				End:   dst.Decorations{"\n", "//dd:endinstrument"},
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

func addInFunctionCode(list []dst.Stmt) []dst.Stmt {
	out := make([]dst.Stmt, 0, len(list))
	for _, stmt := range list {
		appendStmt := true
		switch stmt := stmt.(type) {
		case *dst.AssignStmt:
			// what we actually care about
			// see if it already has a dd:instrumented
			if hasInstrumentedLabel(stmt.Decs.Start.All()) {
				break
			}
			if requestName, ok := analyzeStmtForRequestClient(stmt); ok {
				stmt.Decorations().Start.Prepend("//dd:instrumented")
				out = append(out, stmt)
				appendStmt = false
				out = append(out, buildRequestClientCode(requestName))
			}
			// check for function literal that is a handler
			for pos, expr := range stmt.Rhs {
				if compLit, ok := expr.(*dst.CompositeLit); ok {
					for _, v := range compLit.Elts {
						if kv, ok := v.(*dst.KeyValueExpr); ok {
							if funLit, ok := kv.Value.(*dst.FuncLit); ok {
								if analyzeExpressionForHandlerLiteral(funLit) {
									// get the name from the field
									funLit.Body.List = buildFunctionLiteralHandlerCode(kv.Key, funLit)
								}
								funLit.Body.List = addInFunctionCode(funLit.Body.List)
							}
						}
					}
				}
				if funLit, ok := expr.(*dst.FuncLit); ok {
					if analyzeExpressionForHandlerLiteral(funLit) {
						// get the name from the lhs in the same position -- if it's not there, exit, code isn't going to compile
						if len(stmt.Lhs) <= pos {
							break
						}
						funLit.Body.List = buildFunctionLiteralHandlerCode(stmt.Lhs[pos], funLit)
					}
					funLit.Body.List = addInFunctionCode(funLit.Body.List)
				}
			}
		case *dst.ExprStmt:
			if hasInstrumentedLabel(stmt.Decs.Start.All()) || hasStartInstrumentLabel(stmt.Decs.Start.All()) {
				break
			}
			if wrapped := wrapHandler(stmt); wrapped {
				break
			}

			// might be something we have to recurse on if it's a closure?
			if call, ok := stmt.X.(*dst.CallExpr); ok {
				// check if this is a handler func
				switch funLit := call.Fun.(type) {
				case *dst.FuncLit:
					if analyzeExpressionForHandlerLiteral(funLit) {
						funLit.Body.List = buildFunctionLiteralHandlerCode(nil, funLit)
					}
					funLit.Body.List = addInFunctionCode(funLit.Body.List)
				}
				// check if any of the parameters is a function literal
				var prevExpr dst.Expr
				for _, v := range call.Args {
					if funLit, ok := v.(*dst.FuncLit); ok {
						// check for function literal that is a handler
						if analyzeExpressionForHandlerLiteral(funLit) {
							funLit.Body.List = buildFunctionLiteralHandlerCode(prevExpr, funLit)
						}
					}
					prevExpr = v
				}
			}
		case *dst.GoStmt:
			if funLit, ok := stmt.Call.Fun.(*dst.FuncLit); ok {
				// check for function literal that is a handler
				if analyzeExpressionForHandlerLiteral(funLit) {
					funLit.Body.List = buildFunctionLiteralHandlerCode(nil, funLit)
				}
				funLit.Body.List = addInFunctionCode(funLit.Body.List)
			}
		case *dst.DeferStmt:
			if funLit, ok := stmt.Call.Fun.(*dst.FuncLit); ok {
				// check for function literal that is a handler
				if analyzeExpressionForHandlerLiteral(funLit) {
					funLit.Body.List = buildFunctionLiteralHandlerCode(nil, funLit)
				}
				funLit.Body.List = addInFunctionCode(funLit.Body.List)
			}
		case *dst.BlockStmt:
			stmt.List = addInFunctionCode(stmt.List)
		case *dst.CaseClause:
			stmt.Body = addInFunctionCode(stmt.Body)
		case *dst.CommClause:
			stmt.Body = addInFunctionCode(stmt.Body)
		case *dst.IfStmt:
			stmt.Body.List = addInFunctionCode(stmt.Body.List)
		case *dst.SwitchStmt:
			stmt.Body.List = addInFunctionCode(stmt.Body.List)
		case *dst.TypeSwitchStmt:
			stmt.Body.List = addInFunctionCode(stmt.Body.List)
		case *dst.SelectStmt:
			stmt.Body.List = addInFunctionCode(stmt.Body.List)
		case *dst.ForStmt:
			stmt.Body.List = addInFunctionCode(stmt.Body.List)
		case *dst.RangeStmt:
			stmt.Body.List = addInFunctionCode(stmt.Body.List)
		}
		if appendStmt {
			out = append(out, stmt)
		}
	}
	return out
}

func buildFunctionLiteralHandlerCode(name dst.Expr, funLit *dst.FuncLit) []dst.Stmt {
	//check if magic comment is attached to first line
	if len(funLit.Body.List) > 0 {
		decs := funLit.Body.List[0].Decorations().Start
		for _, v := range decs.All() {
			if strings.HasPrefix(v, "//dd:startinstrument") {
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

func analyzeExpressionForHandlerLiteral(funLit *dst.FuncLit) bool {
	// check the parameters, see if they match
	inputParams := funLit.Type.Params.List
	return len(inputParams) == 2 &&
		IsType(inputParams[0].Type, "net/http", "ResponseWriter") &&
		IsPointerType(inputParams[1].Type, "net/http", "Request")
}

func hasInstrumentedLabel(decs []string) bool {
	isLabeled := false
	for _, v := range decs {
		if strings.HasPrefix(v, "//dd:instrumented") {
			log.Println("already instrumented")
			isLabeled = true
			break
		}
	}
	return isLabeled
}

func hasStartInstrumentLabel(decs []string) bool {
	for _, v := range decs {
		if strings.HasPrefix(v, "//dd:startinstrument") {
			log.Println("already instrumented")
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

func wrapHandler(stmt *dst.ExprStmt) bool {
	if fun, ok := stmt.X.(*dst.CallExpr); ok && len(fun.Args) == 2 {
		if iden, ok := fun.Fun.(*dst.Ident); ok && iden.Path == "net/http" {
			var wrapper string
			switch iden.Name {
			case "Handle":
				wrapper = "WrapHandler"
			case "HandleFunc":
				wrapper = "WrapHandlerFunc"
			default:
				return false
			}
			fun.Decorations().Start.Append("//dd:startinstrument")
			fun.Decorations().End.Append("\n", "//dd:endinstrument")
			fun.Args[1] = &dst.CallExpr{
				Fun:  &dst.Ident{Name: wrapper, Path: "github.com/datadog/orchestrion"},
				Args: []dst.Expr{fun.Args[1]},
			}
			return true
		}
	}
	return false
}

func buildRequestClientCode(requestName string) dst.Stmt {
	/*
		//dd:startinstrument
		if req != nil {
			InsertHeader(req)
			req = req.WithContext(Report(req.Context(), EventCall, "url", req.URL, "method", req.Method))
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
				&dst.ExprStmt{
					X: &dst.CallExpr{
						Fun:  &dst.Ident{Name: "InsertHeader", Path: "github.com/datadog/orchestrion"},
						Args: []dst.Expr{&dst.Ident{Name: requestName}},
					},
				},
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
				Start:  dst.Decorations{"//dd:startinstrument"},
				After:  dst.NewLine,
				End:    dst.Decorations{"\n", "//dd:endinstrument"},
			},
		},
	}
}

func addCodeToHandler(decl *dst.FuncDecl) *dst.FuncDecl {
	//check if magic comment is attached to first line
	if len(decl.Body.List) > 0 {
		decs := decl.Body.List[0].Decorations().Start
		for _, v := range decs.All() {
			if strings.HasPrefix(v, "//dd:startinstrument") {
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
			r = HandleHeader(r)
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
			Rhs: []dst.Expr{&dst.CallExpr{
				Fun:  &dst.Ident{Name: "HandleHeader", Path: "github.com/datadog/orchestrion"},
				Args: []dst.Expr{&dst.Ident{Name: requestName}},
			}},
			Decs: dst.AssignStmtDecorations{NodeDecs: dst.NodeDecs{
				Before: dst.NewLine,
				Start:  dst.Decorations{"//dd:startinstrument"},
				After:  dst.NewLine,
			}},
		},
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
		},
		&dst.DeferStmt{Call: &dst.CallExpr{
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
				End:   dst.Decorations{"\n", "//dd:endinstrument"},
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

func IsPointerType(ex dst.Expr, packageName string, typeName string) bool {
	pointer, ok := ex.(*dst.StarExpr)
	if !ok {
		return false
	}
	return IsType(pointer.X, packageName, typeName)
}

func IsType(ex dst.Expr, packageName string, typeName string) bool {
	selector, ok := ex.(*dst.Ident)
	if !ok {
		return false
	}
	return selector.Name == typeName && selector.Path == packageName
}
