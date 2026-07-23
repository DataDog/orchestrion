// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package join

import (
	"go/token"
	"go/types"

	"github.com/DataDog/orchestrion/internal/injector/aspect/context"
	"github.com/dave/dst"
)

func explicitSlice(node dst.Node) (*dst.SliceExpr, bool) {
	expression, ok := node.(*dst.SliceExpr)
	return expression, ok && expression.Low != nil && expression.High != nil && expression.Max == nil
}

func builtinCall(ctx context.AspectContext, node dst.Node, name string, ellipsis bool) (*dst.CallExpr, bool) {
	call, ok := node.(*dst.CallExpr)
	if !ok || call.Ellipsis != ellipsis || len(call.Args) != 2 {
		return nil, false
	}
	function, ok := call.Fun.(*dst.Ident)
	return call, ok && function.Path == "" && function.Name == name && ctx.IsBuiltin(function)
}

func builtinCallAtLeast(ctx context.AspectContext, node dst.Node, name string, ellipsis bool, arguments int) (*dst.CallExpr, bool) {
	call, ok := node.(*dst.CallExpr)
	if !ok || call.Ellipsis != ellipsis || len(call.Args) < arguments {
		return nil, false
	}
	function, ok := call.Fun.(*dst.Ident)
	return call, ok && function.Path == "" && function.Name == name && ctx.IsBuiltin(function)
}

func unaryBuiltinCall(ctx context.AspectContext, node dst.Node, name string) (*dst.CallExpr, bool) {
	call, ok := node.(*dst.CallExpr)
	if !ok || call.Ellipsis || len(call.Args) != 1 {
		return nil, false
	}
	function, ok := call.Fun.(*dst.Ident)
	return call, ok && function.Path == "" && function.Name == name && ctx.IsBuiltin(function)
}

func isConversion(ctx context.AspectContext, call *dst.CallExpr) bool {
	if len(call.Args) != 1 {
		return false
	}
	target := ctx.ResolveType(call.Fun)
	if target == nil {
		return false
	}
	_, function := target.(*types.Signature)
	return !function
}

func indexedStringConversion(ctx context.AspectContext) (*dst.CallExpr, *dst.IndexExpr, bool) {
	call, ok := ctx.Node().(*dst.CallExpr)
	if !ok || !isConversion(ctx, call) || !isString(ctx.ResolveType(call)) {
		return nil, nil, false
	}
	indexed, ok := call.Args[0].(*dst.IndexExpr)
	return call, indexed, ok
}

func byteAssignment(ctx context.AspectContext) (*dst.AssignStmt, *dst.IndexExpr, bool) {
	assignment, ok := ctx.Node().(*dst.AssignStmt)
	if !ok || assignment.Tok != token.ASSIGN || len(assignment.Lhs) != 1 || len(assignment.Rhs) != 1 {
		return nil, nil, false
	}
	target, ok := assignment.Lhs[0].(*dst.IndexExpr)
	return assignment, target, ok
}

func indexedByteAssignment(ctx context.AspectContext) (*dst.AssignStmt, *dst.IndexExpr, *dst.IndexExpr, bool) {
	assignment, target, ok := byteAssignment(ctx)
	if !ok {
		return nil, nil, nil, false
	}
	source, ok := assignment.Rhs[0].(*dst.IndexExpr)
	return assignment, target, source, ok
}

func scalarExtraction(ctx context.AspectContext) (*dst.AssignStmt, *dst.Ident, *dst.IndexExpr, bool) {
	assignment, ok := ctx.Node().(*dst.AssignStmt)
	if !ok || assignment.Tok != token.DEFINE || len(assignment.Lhs) != 1 || len(assignment.Rhs) != 1 {
		return nil, nil, nil, false
	}
	parent := ctx.Parent()
	if parent == nil {
		return nil, nil, nil, false
	}
	if _, ok := parent.Node().(*dst.BlockStmt); !ok {
		return nil, nil, nil, false
	}
	for ancestor := parent.Parent(); ancestor != nil; ancestor = ancestor.Parent() {
		switch ancestor.Node().(type) {
		case *dst.ForStmt, *dst.RangeStmt:
			return nil, nil, nil, false
		}
	}
	target, ok := assignment.Lhs[0].(*dst.Ident)
	if !ok || target.Name == "_" {
		return nil, nil, nil, false
	}
	source, ok := assignment.Rhs[0].(*dst.IndexExpr)
	if !ok || !repeatableScalarOperand(ctx, source.X) || !repeatableScalarOperand(ctx, source.Index) {
		return nil, nil, nil, false
	}
	return assignment, target, source, true
}

func repeatableScalarOperand(ctx context.AspectContext, expression dst.Expr) bool {
	if _, ok := expression.(*dst.Ident); ok {
		return true
	}
	if call, ok := expression.(*dst.CallExpr); ok && isConversion(ctx, call) && len(call.Args) == 1 {
		return repeatableScalarOperand(ctx, call.Args[0])
	}
	return ctx.IsConstant(expression)
}

func scalarStringConversion(ctx context.AspectContext) (*dst.CallExpr, *dst.Ident, bool) {
	call, ok := ctx.Node().(*dst.CallExpr)
	if !ok || !isConversion(ctx, call) || !isString(ctx.ResolveType(call)) {
		return nil, nil, false
	}
	source, ok := call.Args[0].(*dst.Ident)
	return call, source, ok && ctx.IsAddressable(source)
}
