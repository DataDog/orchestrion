// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

// Package concat provides the shared analysis of Go string concatenation
// expressions (`+` applied to `string` operands) that is used both to select
// join points and to expose the resulting operands to code templates.
//
// The Go compiler lowers a whole chain of `string` additions into a single
// concatenation operation. This package therefore identifies the *maximal*
// chain: the outermost [*dst.BinaryExpr] that is part of one such chain. The
// chain is then flattened into its operands, in source order.
package concat

import (
	"go/constant"
	"go/token"
	"go/types"

	"github.com/dave/dst"

	"github.com/DataDog/orchestrion/internal/injector/typed"
)

// maxFlattenDepth bounds the recursion performed while flattening a
// concatenation chain. A chain deeper than this is not flattened any further,
// and the too-deep subtree is retained as a single operand. This can only occur
// for chains far larger than any supported operand count, which are rejected
// anyway.
const maxFlattenDepth = 64

// Resolver provides the type information needed to analyze a concatenation
// chain. It is satisfied by [context.AspectContext].
type Resolver interface {
	// ResolveType resolves an expression to its type, or nil if unknown.
	ResolveType(dst.Expr) types.Type
	// ResolveConstant resolves an expression to its compile-time constant value,
	// or nil if it is not a constant expression.
	ResolveConstant(dst.Expr) constant.Value
}

// IsStringConcat reports whether expr is a non-constant string concatenation
// expression, that is a `+` binary expression whose core type is `string` and
// which the type-checker did not fold into a constant.
//
// Constant concatenations are excluded because the compiler folds them at
// compile time: there is no run-time operation to advise.
func IsStringConcat(res Resolver, expr dst.Expr) bool {
	bin, ok := expr.(*dst.BinaryExpr)
	if !ok {
		return false
	}
	return IsStringConcatBinary(res, bin)
}

// IsStringConcatBinary is [IsStringConcat] for an already-known
// [*dst.BinaryExpr].
func IsStringConcatBinary(res Resolver, bin *dst.BinaryExpr) bool {
	if bin.Op != token.ADD {
		return false
	}
	if !typed.IsStringCore(res.ResolveType(bin)) {
		return false
	}
	// A constant-folded concatenation is performed by the compiler, not at
	// run-time; it is never part of a run-time concatenation chain.
	return res.ResolveConstant(bin) == nil
}

// Unparen returns expr with all enclosing [*dst.ParenExpr] removed. Parentheses
// are not concatenation boundaries for the compiler, so they are transparent to
// this analysis.
func Unparen(expr dst.Expr) dst.Expr {
	for {
		paren, ok := expr.(*dst.ParenExpr)
		if !ok {
			return expr
		}
		expr = paren.X
	}
}

// Flatten returns the operands of the maximal string concatenation chain rooted
// at root, in source order.
//
// A subexpression that the type-checker folded into a constant is retained as a
// single, atomic operand, matching what the compiler actually concatenates at
// run time. Parentheses are crossed when they enclose a non-constant string
// concatenation, and retained as part of the operand otherwise.
func Flatten(res Resolver, root *dst.BinaryExpr) []dst.Expr {
	// A chain of n operands has n-1 operators; pre-size for a small chain.
	operands := make([]dst.Expr, 0, 4)
	return flatten(res, root, operands, maxFlattenDepth)
}

// flatten appends the operands of the chain rooted at bin to operands, in source
// order, and returns the result.
func flatten(res Resolver, bin *dst.BinaryExpr, operands []dst.Expr, depth int) []dst.Expr {
	for _, operand := range [...]dst.Expr{bin.X, bin.Y} {
		// Cross parentheses only to determine whether the operand continues the
		// chain; the original (possibly parenthesized) expression is retained when
		// it does not.
		if inner, ok := Unparen(operand).(*dst.BinaryExpr); ok &&
			depth > 0 &&
			IsStringConcatBinary(res, inner) {
			operands = flatten(res, inner, operands, depth-1)
			continue
		}
		operands = append(operands, operand)
	}
	return operands
}
