// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package instrument

import "github.com/dave/dst"

func instrumentFiberV2(stmt *dst.AssignStmt) []dst.Stmt {
	return instrumentMiddleware(stmt, isFiberV2, "FiberV2Middleware")
}

func isFiberV2(stmt *dst.AssignStmt) bool {
	rhs := stmt.Rhs[0]
	f, ok := funcIdent(rhs)
	return ok && f.Path == "github.com/gofiber/fiber/v2" && f.Name == "New"
}

// removeFiberV2 returns whether a statement corresponds to orchestrion's Fiber-middleware registration
func removeFiberV2(stmt dst.Stmt) bool {
	return removeUseMiddleware(stmt, "FiberV2Middleware")
}
