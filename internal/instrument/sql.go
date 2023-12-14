// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package instrument

import (
	"fmt"
	"github.com/dave/dst"
	"go/token"
	"strings"
)

func wrapSqlReturnCall(stmt *dst.ReturnStmt) *dst.ReturnStmt {
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
		if wrapSqlCall(fun, stmt.Decorations().Start) {
			markAsWrap(stmt)
			stmt.Decorations().Before = dst.NewLine
		}
	}
	return stmt
}

func wrapSqlOpenFromAssign(stmt *dst.AssignStmt) bool {
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
	if wrapSqlCall(fun, stmt.Decorations().Start) {
		markAsWrap(stmt)
		return true
	}
	return false
}

func wrapSqlCall(call *dst.CallExpr, startDeco dst.Decorations) bool {
	f, ok := call.Fun.(*dst.Ident)

	if !(ok && f.Path == "database/sql" && (f.Name == "Open" || f.Name == "OpenDB")) {
		return false
	}
	f.Path = "github.com/datadog/orchestrion/instrument"
	args := []dst.Expr{
		call.Args[0],
	}
	if f.Name == "Open" {
		args = append(args, call.Args[1])
	}
	for _, dec := range startDeco.All() {
		if strings.HasPrefix(dec, dd_options) {
			optList := strings.Split(strings.TrimPrefix(dec, dd_options), " ")
			for _, opt := range optList {
				optSections := strings.Split(opt, ":")
				if len(optSections) > 1 {
					args = append(args, mapOptionToCall(optSections[0], optSections[1:]...))
				}
			}
		}
	}

	call.Args = args
	return true
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
	if id.Path == "github.com/datadog/orchestrion/instrument" &&
		(id.Name == "Open" || id.Name == "OpenDB") {
		id.Path = "database/sql"
		if id.Name == "Open" {
			f.Args = f.Args[:2]
		} else {
			f.Args = f.Args[:1]
		}
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
	if id.Path == "github.com/datadog/orchestrion/instrument" &&
		(id.Name == "Open" || id.Name == "OpenDB") {
		id.Path = "database/sql"
		if id.Name == "Open" {
			f.Args = f.Args[:2]
		} else {
			f.Args = f.Args[:1]
		}
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
		if !(ok && f.Path == "github.com/datadog/orchestrion/instrument" &&
			(f.Name == "Open" || f.Name == "OpenDB")) {
			continue
		}
		f.Path = "database/sql"
		if f.Name == "Open" {
			fun.Args = fun.Args[:2]
		} else {
			fun.Args = fun.Args[:1]
		}
	}
	return true
}

func mapOptionToCall(name string, value ...string) *dst.CallExpr {
	switch name {
	case "service":
		return &dst.CallExpr{
			Fun: &dst.Ident{Name: "SqlWithServiceName", Path: "github.com/datadog/orchestrion/instrument"},
			Args: []dst.Expr{
				&dst.BasicLit{Kind: token.STRING, Value: fmt.Sprintf(`"%s"`, value[0])},
			},
		}
	case "tag":
		return &dst.CallExpr{
			Fun: &dst.Ident{Name: "SqlWithCustomTag", Path: "github.com/datadog/orchestrion/instrument"},
			Args: []dst.Expr{
				&dst.BasicLit{Kind: token.STRING, Value: fmt.Sprintf(`"%s"`, value[0])},
				&dst.BasicLit{Kind: token.STRING, Value: fmt.Sprintf(`"%s"`, value[1])},
			},
		}
	}
	return nil
}
