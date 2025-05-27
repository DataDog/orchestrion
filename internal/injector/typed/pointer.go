// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package typed

import (
	"github.com/DataDog/orchestrion/internal/fingerprint"
	"github.com/dave/dst"
)

// PointerType represents a pointer to another Go type.
type PointerType struct {
	// Elem is the type that this pointer points to.
	Elem Type
}

// Compile-time check that PointerType implements the Type interface.
var _ Type = (*PointerType)(nil)

// Matches determines whether the provided AST expression node represents
// a pointer to the same type as this PointerType's element type.
func (p *PointerType) Matches(node dst.Expr) bool {
	starExpr, ok := node.(*dst.StarExpr)
	if !ok {
		return false
	}
	return p.Elem.Matches(starExpr.X)
}

// AsNode converts the PointerType back into a dst.Expr AST node.
func (p *PointerType) AsNode() dst.Expr {
	return &dst.StarExpr{X: p.Elem.AsNode()}
}

// Hash contributes the PointerType's properties to a fingerprint hasher.
func (p *PointerType) Hash(h *fingerprint.Hasher) error {
	return h.Named(
		"pointer-type",
		p.Elem,
	)
}
