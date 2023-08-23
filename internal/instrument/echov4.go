// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package instrument

import "github.com/dave/dst"

func isEchoV4(stmt *dst.AssignStmt) bool {
	rhs := stmt.Rhs[0]
	f, ok := funcIdent(rhs)
	return ok && f.Path == "github.com/labstack/echo/v4" && f.Name == "New"
}

func echoV4Middleware(got *dst.AssignStmt) dst.Stmt {
	iden, ok := got.Lhs[0].(*dst.Ident)
	if !ok {
		return nil
	}
	stmt := useMiddleware(iden.Name, "EchoV4Middleware")
	wrap(stmt)
	return stmt
}

// removeEchoV4 returns whether a statement corresponds to orchestrion's Echo-middleware registration
func removeEchoV4(stmt dst.Stmt) bool {
	return removeUseMiddleware(stmt, "EchoV4Middleware")
}
