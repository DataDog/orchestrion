// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package typed

import (
	"github.com/dave/dst"

	"github.com/DataDog/orchestrion/internal/fingerprint"
)

// SliceType represents a slice of another Go type.
type SliceType struct {
	// Elem is the element type of the slice.
	Elem Type
}

// Compile-time check that SliceType implements the Type interface.
var _ Type = (*SliceType)(nil)

// Matches determines whether the provided AST expression node represents
// a slice of the same type as this SliceType's element type.
func (s *SliceType) Matches(node dst.Expr) bool {
	arrayType, ok := node.(*dst.ArrayType)
	if !ok {
		return false
	}
	// A slice has no length specified (Len is nil)
	if arrayType.Len != nil {
		return false
	}
	return s.Elem.Matches(arrayType.Elt)
}

// AsNode converts the SliceType back into a dst.Expr AST node.
func (s *SliceType) AsNode() dst.Expr {
	return &dst.ArrayType{
		Elt: s.Elem.AsNode(),
		// Len is nil for slices
	}
}

// Hash contributes the SliceType's properties to a fingerprint hasher.
func (s *SliceType) Hash(h *fingerprint.Hasher) error {
	return h.Named(
		"slice-type",
		s.Elem,
	)
}

func (s *SliceType) ImportPath() string {
	return s.Elem.ImportPath()
}

func (*SliceType) UnqualifiedName() string {
	// return "[]" + s.Elem.UnqualifiedName()
	return ""
}
