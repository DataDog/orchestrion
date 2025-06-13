// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package typed

import (
	"fmt"
	"regexp"

	"github.com/dave/dst"

	"github.com/DataDog/orchestrion/internal/fingerprint"
)

// Common built-in type definitions for convenience.
// These pre-defined TypeName instances help avoid repeated string literals
// and potential typos when referring to common Go built-in types.
var (
	// Basic types currently used in the codebase
	Any    = MustTypeName("any")
	Bool   = MustTypeName("bool")
	String = MustTypeName("string")
	// Uncomment these when we used.
	// Byte   = MustTypeName("byte")
	// Int    = MustTypeName("int")
	// Error  = MustTypeName("error")
)

// TypeName represents a parsed Go type name, potentially including a package path and pointer indicator.
type TypeName struct {
	// ImportPath is the import Path that provides the type, or an empty string if the
	// type is local or built-in (like "error" or "any").
	ImportPath string
	// Name is the leaf (un-qualified) name of the type.
	Name string
	// Pointer determines whether the specified type is a pointer or not.
	Pointer bool
}

// FIXME: this does not support all the type syntax, like: "chan Event"
// It primarily handles identifiers, qualified identifiers, and pointers to those.
var typeNameRe = regexp.MustCompile(`\A(\*)?\s*(?:([A-Za-z0-9_.-]+(?:/[A-Za-z0-9_.-]+)*)\.)?([A-Za-z_][A-Za-z0-9_]*)\z`)

// NewTypeName parses a string representation of a type name into a TypeName struct.
// It returns an error if the syntax is invalid according to its limited regular expression.
func NewTypeName(n string) (tn TypeName, err error) {
	matches := typeNameRe.FindStringSubmatch(n)
	if matches == nil {
		err = fmt.Errorf("invalid TypeName syntax: %q", n)
		return tn, err
	}

	tn.Pointer = matches[1] == "*"
	tn.ImportPath = matches[2]
	tn.Name = matches[3]
	return tn, nil
}

// MustTypeName is the same as NewTypeName, except it panics in case of an error.
func MustTypeName(n string) (tn TypeName) {
	var err error
	if tn, err = NewTypeName(n); err != nil {
		panic(err)
	}
	return tn
}

// Matches determines whether the provided AST expression node represents the same type
// as this TypeName. This performs a structural comparison based on the limited types
// supported by the parsing regex (identifiers, selectors, pointers, empty interface).
func (n TypeName) Matches(node dst.Expr) bool {
	switch node := node.(type) {
	case *dst.Ident:
		return !n.Pointer && n.ImportPath == node.Path && n.Name == node.Name

	case *dst.SelectorExpr:
		var path string
		if ident, ok := node.X.(*dst.Ident); ok && ident.Path == "" {
			path = ident.Name
		} else {
			return false
		}
		return !n.Pointer && n.ImportPath == path && n.Name == node.Sel.Name

	case *dst.StarExpr:
		return n.Pointer && (&TypeName{ImportPath: n.ImportPath, Name: n.Name}).Matches(node.X)

	case *dst.IndexExpr:
		// Handle generic types with single type parameter (e.g., MyType[T])
		return !n.Pointer && n.Matches(node.X)

	case *dst.IndexListExpr:
		// Handle generic types with multiple type parameters (e.g., MyType[T, U])
		return !n.Pointer && n.Matches(node.X)

	case *dst.InterfaceType:
		// We only match the empty interface (as "any")
		if len(node.Methods.List) != 0 {
			return false
		}
		return n.ImportPath == "" && n.Name == "any"

	default:
		return false
	}
}

// MatchesDefinition determines whether the provided node matches the definition
// of this TypeName. The `importPath` argument determines the context in which
// the assertion is made.
func (n TypeName) MatchesDefinition(node dst.Expr, importPath string) bool {
	if n.ImportPath != importPath {
		return false
	}
	return (&TypeName{Name: n.Name, Pointer: n.Pointer}).Matches(node)
}

// AsNode converts the TypeName back into a dst.Expr AST node.
// Useful for generating code that refers to this type.
func (n *TypeName) AsNode() dst.Expr {
	ident := dst.NewIdent(n.Name)
	ident.Path = n.ImportPath
	if n.Pointer {
		return &dst.StarExpr{X: ident}
	}
	return ident
}

// Hash contributes the TypeName's properties to a fingerprint hasher.
func (n TypeName) Hash(h *fingerprint.Hasher) error {
	return h.Named(
		"type-name",
		fingerprint.String(n.Name),
		fingerprint.String(n.ImportPath),
		fingerprint.Bool(n.Pointer),
	)
}

// FindMatchingTypeName parses a type name string and searches a field list for the first field whose type matches.
// It returns the index of the matching field and whether a match was found.
// The index accounts for fields with multiple names.
func FindMatchingTypeName(fields *dst.FieldList, typeNameStr string) (index int, found bool) {
	if fields == nil || len(fields.List) == 0 {
		return -1, false
	}

	tn, err := NewTypeName(typeNameStr)
	if err != nil {
		// If the type name string is invalid, we can't match it.
		return -1, false
	}

	currentIndex := 0
	for _, field := range fields.List {
		if tn.Matches(field.Type) {
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
