// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package typed

import (
	"github.com/dave/dst"

	"github.com/DataDog/orchestrion/internal/fingerprint"
)

// Type represents a Go type that can be matched against AST nodes,
// converted to AST nodes, and hashed for fingerprinting.
//
// Currently supported type implementations:
//   - NamedType: Basic types (e.g., "string", "int") and qualified types (e.g., "net/http.Request")
//   - PointerType: Pointer types (e.g., "*string", "*net/http.Request")
//   - SliceType: Slice types (e.g., "[]string", "[]*User")
//   - ArrayType: Array types with fixed size (e.g., "[10]string", "[0xFF]byte")
//   - MapType: Map types (e.g., "map[string]int", "map[string]*User")
//
// Types not yet supported (future work):
//   - Channel types: "chan int", "<-chan int", "chan<- int"
//   - Function types: "func()", "func(int, string) bool"
//   - Interface types: "interface{}", "interface{ Method() string }"
//   - Struct types: "struct{}", "struct{ Name string; Age int }"
//   - Generic types: "List[T]", "Map[K comparable, V any]"
//
// These unsupported types are less commonly used in dependency injection scenarios,
// which is why they were not prioritized in the initial implementation.
type Type interface {
	// Matches determines whether the provided AST expression node represents
	// the same type as this Type instance.
	Matches(node dst.Expr) bool

	// AsNode converts the Type back into a dst.Expr AST node.
	// Useful for generating code that refers to this type.
	AsNode() dst.Expr

	// Hash contributes the Type's properties to a fingerprint hasher.
	Hash(h *fingerprint.Hasher) error
}
