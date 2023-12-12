// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package instrument

import "github.com/dave/dst"

func instrumentGin(stmt *dst.AssignStmt) []dst.Stmt {
	return instrumentMiddleware(stmt, isGin, "GinMiddleware")
}

func isGin(stmt *dst.AssignStmt) bool {
	rhs := stmt.Rhs[0]
	f, ok := funcIdent(rhs)
	return ok && f.Path == "github.com/gin-gonic/gin" && (f.Name == "New" || f.Name == "Default")
}

// removeGin returns whether a statement corresponds to orchestrion's Gin-middleware registration
func removeGin(stmt dst.Stmt) bool {
	return removeUseMiddleware(stmt, "GinMiddleware")
}
