package orchestrion

import (
	"bytes"
	"fmt"
	"go/token"
	"io"
	"strings"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
	"github.com/dave/dst/decorator/resolver/goast"
	"github.com/dave/dst/decorator/resolver/guess"
)

var unwrappers = []func(n dst.Node) bool{
	unwrapClient,
	unwrapHandlerExpr,
	unwrapHandlerAssign,
	unwrapSqlExpr,
	unwrapSqlAssign,
	unwrapSqlReturn,
}

func UninstrumentFile(name string, r io.Reader, conf Config) (io.Reader, error) {
	fset := token.NewFileSet()
	d := decorator.NewDecoratorWithImports(fset, name, goast.New())
	f, err := d.Parse(r)
	if err != nil {
		return nil, fmt.Errorf("error parsing content in %s: %w", name, err)
	}

	for _, decl := range f.Decls {
		if decl, ok := decl.(*dst.FuncDecl); ok {
			decl.Body.List = removeStartEndWrap(decl.Body.List)
			decl.Body.List = removeStartEndInstrument(decl.Body.List)
			// recurse for function literals
			for _, stmt := range decl.Body.List {
				switch stmt := stmt.(type) {
				case *dst.AssignStmt:
					for _, expr := range stmt.Rhs {
						if compLit, ok := expr.(*dst.CompositeLit); ok {
							for _, v := range compLit.Elts {
								if kv, ok := v.(*dst.KeyValueExpr); ok {
									if funLit, ok := kv.Value.(*dst.FuncLit); ok {
										funLit.Body.List = removeStartEndWrap(funLit.Body.List)
										funLit.Body.List = removeStartEndInstrument(funLit.Body.List)
									}
								}
							}
						}
						if funLit, ok := expr.(*dst.FuncLit); ok {
							funLit.Body.List = removeStartEndWrap(funLit.Body.List)
							funLit.Body.List = removeStartEndInstrument(funLit.Body.List)
						}
					}
				case *dst.ExprStmt:
					if call, ok := stmt.X.(*dst.CallExpr); ok {
						switch funLit := call.Fun.(type) {
						case *dst.FuncLit:
							funLit.Body.List = removeStartEndWrap(funLit.Body.List)
							funLit.Body.List = removeStartEndInstrument(funLit.Body.List)
						}
					}
				}
			}
		}
	}

	res := decorator.NewRestorerWithImports(name, guess.New())
	var out bytes.Buffer
	err = res.Fprint(&out, f)
	return &out, err
}

func removeDecl(prefix string, ds dst.Decorations) []string {
	var rds []string
	for i := range ds {
		if strings.HasPrefix(ds[i], prefix) {
			continue
		}
		rds = append(rds, ds[i])
	}
	return rds
}

func removeDecoration(deco string, s dst.Stmt) {
	s.Decorations().Start.Replace(removeDecl(deco, s.Decorations().Start)...)
	s.Decorations().End.Replace(removeDecl(deco, s.Decorations().End)...)
}

func removeStartEndWrap(list []dst.Stmt) []dst.Stmt {
	for {
		var start, end int
		var found bool
		for i, stmt := range list {
			if hasLabel(dd_endwrap, stmt.Decorations().Start.All()) {
				stmt.Decorations().Start.Replace(
					removeDecl(dd_endwrap, stmt.Decorations().Start)...)
				end = i
				found = true
				break
			} else if hasLabel(dd_startwrap, stmt.Decorations().Start.All()) {
				stmt.Decorations().Start.Replace(
					removeDecl(dd_startwrap, stmt.Decorations().Start)...)
				start = i
				if hasLabel(dd_endwrap, stmt.Decorations().End.All()) {
					stmt.Decorations().End.Replace(
						removeDecl(dd_endwrap, stmt.Decorations().End)...)
					end = i + 1
					found = true
					break
				}
			}
		}
		if !found {
			// Never found a start/end pair.
			return list
		}

		for _, s := range list[start:end] {
			for _, unwrap := range unwrappers {
				dst.Inspect(s, unwrap)
			}
		}
	}
}

func removeStartEndInstrument(list []dst.Stmt) []dst.Stmt {
	var start, end int
	for i, stmt := range list {
		if hasLabel(dd_startinstrument, stmt.Decorations().Start.All()) {
			start = i
		}
		if hasLabel(dd_endinstrument, stmt.Decorations().Start.All()) {
			end = i
			removeDecoration(dd_endinstrument, list[end])
			list = append(list[:start], list[end:]...)
			// We found one. There may be others, so recurse.
			// We can make this more efficient...
			return removeStartEndInstrument(list)
		}
		if hasLabel(dd_endinstrument, stmt.Decorations().End.All()) {
			list = list[:start]
			// We found one. There may be others, so recurse.
			// We can make this more efficient...
			return removeStartEndInstrument(list)
		}
		if hasLabel(dd_instrumented, stmt.Decorations().Start.All()) {
			removeDecoration(dd_instrumented, stmt)
		}
	}
	return list
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
	if cei.Path == "github.com/datadog/orchestrion" && strings.HasPrefix(cei.Name, "WrapHTTPClient") {
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
	if len(f.Args) > 1 {
		if ce, ok := f.Args[1].(*dst.CallExpr); ok {
			if cei, ok := ce.Fun.(*dst.Ident); ok {
				if cei.Path == "github.com/datadog/orchestrion" &&
					strings.HasPrefix(cei.Name, "WrapHandler") {
					// This catches both WrapHandler *and* WrapHandlerFunc
					f.Args[1] = ce.Args[0]
					return false
				}
			}
		}
	}
	return true
}

// unwrapHandlerAssign unwraps handler assignements, to be used in dst.Inspect.
// Returns true to continue the traversal, false to stop.
func unwrapHandlerAssign(n dst.Node) bool {
	// TODO: Implement me
	return false
}

func unwrapSqlExpr(n dst.Node) bool {
	es, ok := n.(*dst.ExprStmt)
	if !ok {
		return true
	}
	f, ok := es.X.(*dst.CallExpr)
	if !ok {
		return true
	}
	id, ok := f.Fun.(*dst.Ident)
	if !ok {
		return true
	}
	if id.Path == "github.com/datadog/orchestrion/sql" &&
		(id.Name == "Open" || id.Name == "OpenDB") {
		id.Path = "database/sql"
		return true
	}
	return true
}

func unwrapSqlAssign(n dst.Node) bool {
	as, ok := n.(*dst.AssignStmt)
	if !ok {
		return true
	}
	f, ok := as.Rhs[0].(*dst.CallExpr)
	if !ok {
		return true
	}
	id, ok := f.Fun.(*dst.Ident)
	if !ok {
		return true
	}
	if id.Path == "github.com/datadog/orchestrion/sql" &&
		(id.Name == "Open" || id.Name == "OpenDB") {
		id.Path = "database/sql"
		return true
	}
	return true
}

func unwrapSqlReturn(n dst.Node) bool {
	rs, ok := n.(*dst.ReturnStmt)
	if !ok {
		return true
	}
	for _, expr := range rs.Results {
		fun, ok := expr.(*dst.CallExpr)
		if !ok {
			continue
		}
		f, ok := fun.Fun.(*dst.Ident)
		if !(ok && f.Path == "github.com/datadog/orchestrion/sql" &&
			(f.Name == "Open" || f.Name == "OpenDB")) {
			continue
		}
		f.Path = "database/sql"
	}
	return true
}
