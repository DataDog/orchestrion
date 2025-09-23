// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package typed

import (
	"fmt"

	"github.com/dave/dst"

	"github.com/DataDog/orchestrion/internal/fingerprint"
)

// Common built-in type definitions for convenience.
// These pre-defined NamedType instances help avoid repeated string literals
// and potential typos when referring to common Go built-in types.
var (
	// Basic types currently used in the codebase
	Any    = &NamedType{Path: "", Name: "any"}
	Bool   = &NamedType{Path: "", Name: "bool"}
	String = &NamedType{Path: "", Name: "string"}
	// Uncomment these when needed.
	// Byte   = &NamedType{ImportPath: "", Name: "byte"}
	// Int    = &NamedType{ImportPath: "", Name: "int"}
	// Error  = &NamedType{ImportPath: "", Name: "error"}
)

// NamedType represents a parsed Go type name, potentially including a package path.
type NamedType struct {
	// Path is the import Path that provides the type, or an empty string if the
	// type is local or built-in (like "error" or "any").
	Path string
	// Name is the leaf (un-qualified) name of the type.
	Name string
}

// Compile-time check that NamedType implements the Type interface.
var _ Type = (*NamedType)(nil)

// NewNamedType parses a string representation of a type name into a NamedType struct.
// It supports pointer types by automatically unwrapping them to get the underlying named type.
// This is a convenience function that combines NewType and ExtractNamedType.
func NewNamedType(n string) (*NamedType, error) {
	t, err := NewType(n)
	if err != nil {
		return nil, err
	}
	return ExtractNamedType(t)
}

// MustNamedType is the same as NewNamedType, except it panics in case of an error.
func MustNamedType(n string) *NamedType {
	nt, err := NewNamedType(n)
	if err != nil {
		panic(err)
	}
	return nt
}

// Matches determines whether the provided AST expression node represents the same type
// as this NamedType. This performs a structural comparison based on the limited types
// supported by the parsing regex (identifiers, selectors, empty interface).
func (n *NamedType) Matches(node dst.Expr) bool {
	switch node := node.(type) {
	case *dst.Ident:
		return n.Path == node.Path && n.Name == node.Name

	case *dst.SelectorExpr:
		var path string
		if ident, ok := node.X.(*dst.Ident); ok && ident.Path == "" {
			path = ident.Name
		} else {
			return false
		}
		return n.Path == path && n.Name == node.Sel.Name

	case *dst.StarExpr:
		// NamedType should not match pointer expressions
		return false

	case *dst.IndexExpr:
		// Handle generic types with single type parameter (e.g., MyType[T])
		return n.Matches(node.X)

	case *dst.IndexListExpr:
		// Handle generic types with multiple type parameters (e.g., MyType[T, U])
		return n.Matches(node.X)

	case *dst.InterfaceType:
		// We only match the empty interface (as "any")
		if len(node.Methods.List) != 0 {
			return false
		}
		return n.Path == "" && n.Name == "any"

	default:
		return false
	}
}

// MatchesDefinition determines whether the provided node matches the definition
// of this NamedType. The `importPath` argument determines the context in which
// the assertion is made.
func (n *NamedType) MatchesDefinition(node dst.Expr, importPath string) bool {
	if n.Path != importPath {
		return false
	}
	return (&NamedType{Name: n.Name}).Matches(node)
}

// AsNode converts the NamedType back into a dst.Expr AST node.
// Useful for generating code that refers to this type.
func (n *NamedType) AsNode() dst.Expr {
	ident := dst.NewIdent(n.Name)
	ident.Path = n.Path
	return ident
}

// Hash contributes the NamedType's properties to a fingerprint hasher.
func (n NamedType) Hash(h *fingerprint.Hasher) error {
	return h.Named(
		"named-type",
		fingerprint.String(n.Name),
		fingerprint.String(n.Path),
	)
}

// ImportPath is the import Path that provides the type, or an empty string if the
// type is local or built-in (like "error" or "any").
func (n NamedType) ImportPath() string {
	return n.Path
}

func (n NamedType) UnqualifiedName() string {
	return n.Name
}

// FindMatchingType parses a type string and searches a field list for the first field whose type matches.
// It returns the index of the matching field and whether a match was found.
// The index accounts for fields with multiple names.
func FindMatchingType(fields *dst.FieldList, typeStr string) (index int, found bool) {
	if fields == nil || len(fields.List) == 0 {
		return -1, false
	}

	t, err := NewType(typeStr)
	if err != nil {
		// If the type string is invalid, we can't match it.
		return -1, false
	}

	currentIndex := 0
	for _, field := range fields.List {
		if t.Matches(field.Type) {
			return currentIndex, true // Found a match.
		}

		// Increment index by the number of names in the field (or 1 if unnamed).
		count := len(field.Names)
		if count == 0 {
			count = 1
		}
		currentIndex += count
	}

	return -1, false // No match found
}

// AsNamedType extracts a NamedType from a Type interface.
// Returns an error if the Type is not a NamedType.
func AsNamedType(t Type) (NamedType, error) {
	switch v := t.(type) {
	case *NamedType:
		return *v, nil
	default:
		return NamedType{}, fmt.Errorf("cannot convert %T to NamedType", t)
	}
}
