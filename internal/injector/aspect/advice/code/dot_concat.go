// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package code

import (
	"errors"

	"github.com/DataDog/orchestrion/internal/injector/aspect/concat"
)

// errNotStringConcat is returned by [dot.StringConcatOperands] when the current
// node is not part of a non-constant `string` concatenation chain.
var errNotStringConcat = errors.New("the node in this context is not part of a string concatenation chain")

// StringConcatOperands returns the operands of the maximal `string`
// concatenation chain the current node belongs to, in source order.
//
// Parentheses do not split a concatenation chain, so `(a + b) + c` yields three
// operands. A subexpression the compiler folds into a constant is retained as a
// single operand, so `a + ("x" + "y")` yields two operands: `a` and `("x" + "y")`.
//
// Each returned value renders as a placeholder replaced by the original operand
// expression and keeps its original type. A template using this helper MUST emit
// every operand exactly once and in source order. Omitting an operand drops its
// side effects; copying or rendering one more than once repeats its evaluation.
//
// This fails if the current node is not part of a non-constant `string`
// concatenation chain, which normally means the advice was not attached to a
// `string-concat` join point.
func (d *dot) StringConcatOperands() ([]any, error) {
	_, operands := concat.RootOperands(d.context)
	if operands == nil {
		return nil, errNotStringConcat
	}

	res := make([]any, len(operands))
	for i, operand := range operands {
		res[i] = newProxy[any](operand, &d.placeholders)
	}

	return res, nil
}
