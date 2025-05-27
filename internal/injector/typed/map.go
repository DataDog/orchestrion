// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package typed

import (
	"github.com/dave/dst"

	"github.com/DataDog/orchestrion/internal/fingerprint"
)

// MapType represents a map from one Go type to another.
type MapType struct {
	// Key is the key type of the map.
	Key Type
	// Value is the value type of the map.
	Value Type
}

// Compile-time check that MapType implements the Type interface.
var _ Type = (*MapType)(nil)

// Matches determines whether the provided AST expression node represents
// a map with the same key and value types as this MapType.
func (m *MapType) Matches(node dst.Expr) bool {
	mapType, ok := node.(*dst.MapType)
	if !ok {
		return false
	}

	if !m.Key.Matches(mapType.Key) {
		return false
	}

	return m.Value.Matches(mapType.Value)
}

// AsNode converts the MapType back into a dst.Expr AST node.
func (m *MapType) AsNode() dst.Expr {
	return &dst.MapType{
		Key:   m.Key.AsNode(),
		Value: m.Value.AsNode(),
	}
}

// Hash contributes the MapType's properties to a fingerprint hasher.
func (m *MapType) Hash(h *fingerprint.Hasher) error {
	return h.Named(
		"map-type",
		m.Key,
		m.Value,
	)
}
