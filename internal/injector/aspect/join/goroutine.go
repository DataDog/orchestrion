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
	"github.com/DataDog/orchestrion/internal/yaml"
	"github.com/dave/dst"
	"github.com/goccy/go-yaml/ast"
)

type goStatementCall bool

// GoStatementCall matches the *dst.CallExpr operand of a `go` statement
// (i.e. the `f(args...)` in `go f(args...)`) if v is true, or any node that
// is not such a call expression if v is false -- mirroring [TestMain]'s
// true-matches/false-matches-the-complement convention for boolean-flag
// join points. Routing through the call expression, rather than the `go`
// statement itself, allows this join point to be paired with any advice
// that operates on a dst.Expr, such as wrap-expression.
func GoStatementCall(v bool) goStatementCall {
	return goStatementCall(v)
}

func (goStatementCall) ImpliesImported() []string {
	return nil
}

func (goStatementCall) PackageMayMatch(_ *may.PackageContext) may.MatchType {
	return may.Match
}

func (goStatementCall) FileMayMatch(_ *may.FileContext) may.MatchType {
	// A `go` statement's call need not be adjacent to the literal text "go "
	// (e.g. `go\n\tf()` is valid Go), so no reliable substring exists to
	// search for here.
	return may.Unknown
}

func (t goStatementCall) Matches(ctx context.AspectContext) bool {
	return t.isGoStatementCall(ctx) == bool(t)
}

func (goStatementCall) isGoStatementCall(ctx context.AspectContext) bool {
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

func (t goStatementCall) Hash(h *fingerprint.Hasher) error {
	return h.Named("go-statement-call", fingerprint.Bool(t))
}

func init() {
	unmarshalers["go-statement-call"] = func(ctx gocontext.Context, node ast.Node) (Point, error) {
		var val bool
		if err := yaml.NodeToValueContext(ctx, node, &val); err != nil {
			return nil, err
		}
		return GoStatementCall(val), nil
	}
}
