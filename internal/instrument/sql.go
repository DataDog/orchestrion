// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package instrument

import "github.com/dave/dst"

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
		if wrapSqlCall(fun) {
			wrap(stmt)
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
	if wrapSqlCall(fun) {
		wrap(stmt)
		return true
	}
	return false
}

func wrapSqlCall(call *dst.CallExpr) bool {
	f, ok := call.Fun.(*dst.Ident)
	if !(ok && f.Path == "database/sql" && (f.Name == "Open" || f.Name == "OpenDB")) {
		return false
	}
	f.Path = "github.com/datadog/orchestrion/instrument"
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
	}
	return true
}
