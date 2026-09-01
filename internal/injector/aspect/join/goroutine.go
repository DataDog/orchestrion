// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package join

import (
	gocontext "context"

	"github.com/DataDog/orchestrion/internal/fingerprint"
	"github.com/DataDog/orchestrion/internal/injector/aspect/context"
	"github.com/DataDog/orchestrion/internal/injector/aspect/may"
	"github.com/dave/dst"
	"github.com/goccy/go-yaml/ast"
)

type goStatementCall struct{}

// GoStatementCall matches the *dst.CallExpr operand of a `go` statement
// (i.e. the `f(args...)` in `go f(args...)`). Routing through the call
// expression, rather than the `go` statement itself, allows this join point
// to be paired with any advice that operates on a dst.Expr, such as
// wrap-expression.
func GoStatementCall() *goStatementCall {
	return &goStatementCall{}
}

func (*goStatementCall) ImpliesImported() []string {
	return nil
}

func (*goStatementCall) PackageMayMatch(_ *may.PackageContext) may.MatchType {
	return may.Match
}

func (*goStatementCall) FileMayMatch(_ *may.FileContext) may.MatchType {
	// A `go` statement's call need not be adjacent to the literal text "go "
	// (e.g. `go\n\tf()` is valid Go), so no reliable substring exists to
	// search for here.
	return may.Unknown
}

func (*goStatementCall) Matches(ctx context.AspectContext) bool {
	call, ok := ctx.Node().(*dst.CallExpr)
	if !ok {
		return false
	}

	parent := ctx.Parent()
	if parent == nil {
		return false
	}
	defer parent.Release()

	goStmt, ok := parent.Node().(*dst.GoStmt)
	return ok && goStmt.Call == call
}

func (*goStatementCall) Hash(h *fingerprint.Hasher) error {
	return h.Named("go-statement-call")
}

func init() {
	unmarshalers["go-statement-call"] = func(_ gocontext.Context, _ ast.Node) (Point, error) {
		return GoStatementCall(), nil
	}
}
