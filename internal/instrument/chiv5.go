// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package instrument

import (
	"github.com/dave/dst"
)

func instrumentChiV5(stmt *dst.AssignStmt) []dst.Stmt {
	return instrumentMiddleware(stmt, isChiV5, "ChiV5Middleware")
}

func isChiV5(stmt *dst.AssignStmt) bool {
	rhs := stmt.Rhs[0]
	f, ok := funcIdent(rhs)
	return ok && f.Path == "github.com/go-chi/chi/v5" && f.Name == "NewRouter"
}

// removeChiV5 returns whether a statement corresponds to orchestrion's Chi-middleware registration
func removeChiV5(stmt dst.Stmt) bool {
	return removeUseMiddleware(stmt, "ChiV5Middleware")
}
