// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package concat

import (
	"github.com/dave/dst"

	"github.com/DataDog/orchestrion/internal/injector/aspect/context"
)

// Root returns the root of the maximal string concatenation chain the context's
// current node belongs to, or nil if that node is not part of a non-constant
// string concatenation chain.
//
// The search crosses [*dst.ParenExpr] ancestors, as parentheses do not split a
// concatenation chain as far as the compiler is concerned. The returned node is
// the outermost [*dst.BinaryExpr] of the chain.
func Root(ctx context.AspectContext) *dst.BinaryExpr {
	node := ctx.Node()

	// The starting point must itself be part of a concatenation chain. Parentheses
	// around it are transparent.
	expr, ok := node.(dst.Expr)
	if !ok {
		return nil
	}
	root, ok := Unparen(expr).(*dst.BinaryExpr)
	if !ok || !IsStringConcatBinary(ctx, root) {
		return nil
	}

	// Walk outwards for as long as ancestors keep extending the same chain.
	for curr := ctx.Chain(); curr != nil; curr = curr.Parent() {
		if curr.Node() == node {
			// This is the level of the starting node itself.
			continue
		}

		switch parent := curr.Node().(type) {
		case *dst.ParenExpr:
			// Parentheses are transparent; keep looking outwards.
			continue
		case *dst.BinaryExpr:
			if !IsStringConcatBinary(ctx, parent) {
				// A different operation (such as a comparison): the chain ends here.
				return root
			}
			root = parent
		default:
			// Any other node type ends the chain.
			return root
		}
	}

	return root
}

// IsRoot reports whether the context's current node is the root of a maximal
// non-constant string concatenation chain.
func IsRoot(ctx context.AspectContext) bool {
	root := Root(ctx)
	return root != nil && root == ctx.Node()
}

// RootOperands returns the root of the maximal string concatenation chain the
// context's current node belongs to, together with that chain's operands in
// source order. The root is nil if the current node is not part of a
// non-constant string concatenation chain.
func RootOperands(ctx context.AspectContext) (*dst.BinaryExpr, []dst.Expr) {
	root := Root(ctx)
	if root == nil {
		return nil, nil
	}
	return root, Flatten(ctx, root)
}
