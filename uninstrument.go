// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

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
	var start, end int
	var found bool
	for i, stmt := range list {
		if hasLabel(dd_startwrap, stmt.Decorations().Start.All()) {
			start = i
			if hasLabel(dd_endwrap, stmt.Decorations().End.All()) {
				end = i
				found = true
				break
			}
		} else if hasLabel(dd_endwrap, stmt.Decorations().Start.All()) {
			end = i
			found = true
			break
		}
	}
	if !found {
		// Never found a start/end pair.
		return list
	}
	removeDecoration(dd_endwrap, list[end])
	removeDecoration(dd_startwrap, list[start])
	for _, s := range list {
		for _, unwrap := range unwrappers {
			dst.Inspect(s, unwrap)
		}
	}
	return list
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
