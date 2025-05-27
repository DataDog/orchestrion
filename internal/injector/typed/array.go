// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package typed

import (
	"go/token"
	"strconv"

	"github.com/DataDog/orchestrion/internal/fingerprint"
	"github.com/dave/dst"
)

// ArrayType represents an array of a fixed size of another Go type.
type ArrayType struct {
	// Size is the fixed size of the array.
	Size int
	// Elem is the element type of the array.
	Elem Type
}

// Compile-time check that ArrayType implements the Type interface.
var _ Type = (*ArrayType)(nil)

// Matches determines whether the provided AST expression node represents
// an array of the same size and type as this ArrayType.
func (a *ArrayType) Matches(node dst.Expr) bool {
	arrayType, ok := node.(*dst.ArrayType)
	if !ok {
		return false
	}
	// An array must have a length specified
	if arrayType.Len == nil {
		return false
	}

	// Check if the length matches
	lit, ok := arrayType.Len.(*dst.BasicLit)
	if !ok {
		return false
	}

	// Parse the size from the literal with platform-appropriate bit size
	size, err := strconv.ParseInt(lit.Value, 0, strconv.IntSize)
	if err != nil {
		return false
	}

	// No need for bounds checking since we parsed with IntSize
	if int(size) != a.Size {
		return false
	}

	return a.Elem.Matches(arrayType.Elt)
}

// AsNode converts the ArrayType back into a dst.Expr AST node.
func (a *ArrayType) AsNode() dst.Expr {
	return &dst.ArrayType{
		Len: &dst.BasicLit{
			Kind:  token.INT,
			Value: strconv.Itoa(a.Size),
		},
		Elt: a.Elem.AsNode(),
	}
}

// Hash contributes the ArrayType's properties to a fingerprint hasher.
func (a *ArrayType) Hash(h *fingerprint.Hasher) error {
	return h.Named(
		"array-type",
		fingerprint.Int(a.Size),
		a.Elem,
	)
}
