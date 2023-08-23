// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package instrument

import "github.com/dave/dst"

func isGin(stmt *dst.AssignStmt) bool {
	rhs := stmt.Rhs[0]
	f, ok := funcIdent(rhs)
	return ok && f.Path == "github.com/gin-gonic/gin" && (f.Name == "New" || f.Name == "Default")
}

func ginMiddleware(got *dst.AssignStmt) dst.Stmt {
	iden, ok := got.Lhs[0].(*dst.Ident)
	if !ok {
		return nil
	}
	stmt := useMiddleware(iden.Name, "GinMiddleware")
	wrap(stmt)
	return stmt
}

// removeGin returns whether a statement corresponds to orchestrion's Gin-middleware registration
func removeGin(stmt dst.Stmt) bool {
	return removeUseMiddleware(stmt, "GinMiddleware")
}
