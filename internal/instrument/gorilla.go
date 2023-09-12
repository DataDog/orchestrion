// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package instrument

import "github.com/dave/dst"

func wrapGorillaMux(stmt *dst.AssignStmt) {
	rhs := stmt.Rhs[0]
	f, ok := funcIdent(rhs)
	if !ok {
		return
	}
	if !(f.Path == "github.com/gorilla/mux" && f.Name == "NewRouter") {
		return
	}
	markAsWrap(stmt)
	call := rhs.(*dst.CallExpr)
	call.Args = []dst.Expr{
		&dst.CallExpr{
			Fun: &dst.Ident{
				Name: f.Name,
				Path: f.Path,
			},
		},
	}
	f.Path = "github.com/datadog/orchestrion/instrument"
	f.Name = "WrapGorillaMuxRouter"
}

func unwrapGorillaMux(n dst.Node) bool {
	stmt, ok := n.(*dst.AssignStmt)
	if !ok {
		return true
	}
	rhs := stmt.Rhs[0]
	f, ok := funcIdent(rhs)
	if !ok {
		return true
	}
	if !(f.Path == "github.com/datadog/orchestrion/instrument" && f.Name == "WrapGorillaMuxRouter") {
		return true
	}
	call := rhs.(*dst.CallExpr)
	call.Args = nil
	f.Path = "github.com/gorilla/mux"
	f.Name = "NewRouter"
	return true
}
