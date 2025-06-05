// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package typed

import (
	"fmt"
	"go/importer"
	"go/types"
	"strings"

	"github.com/dave/dst"
)

// TypeResolver defines the capability to resolve a dst expression to its go/types type.
type TypeResolver interface {
	ResolveType(dst.Expr) types.Type
}

// ExprImplements checks if the type of a dst.Expr, resolved using the provider,
// implements the given interface.
func ExprImplements(resolver TypeResolver, expr dst.Expr, iface *types.Interface) bool {
	actualType := resolver.ResolveType(expr)
	if actualType == nil {
		return false
	}
	return typeImplements(actualType, iface)
}

// typeImplements checks if a type implements an interface.
func typeImplements(t types.Type, iface *types.Interface) bool {
	if t == nil || iface == nil {
		return false
	}

	// Direct implementation check.
	if types.Implements(t, iface) {
		return true
	}

	// types.Implements handles pointer receivers implicitly, so no need for explicit pointer check.
	return false
}

// ResolveInterfaceTypeByName takes an interface name as a string and resolves it to an interface type.
func ResolveInterfaceTypeByName(name string) (*types.Interface, error) {
	// Handle built-in types.
	if obj := types.Universe.Lookup(name); obj != nil {
		typeObj, ok := obj.(*types.TypeName)
		if !ok {
			return nil, fmt.Errorf("object %s is not a type name but a %T", name, obj)
		}

		typ := typeObj.Type()
		if !types.IsInterface(typ) {
			return nil, fmt.Errorf("type %s is not an interface", name)
		}

		t, ok := typ.Underlying().(*types.Interface)
		if !ok {
			return nil, fmt.Errorf("type %s is not an interface", name)
		}

		return t, nil
	}

	// Handle package-qualified types (e.g., "io.Writer").
	pkgPath, typeName := SplitPackageAndName(name)
	if pkgPath == "" {
		// If not built-in and no package path, it's likely an undefined or local type
		// that importer won't find directly without context. Assume invalid for now.
		return nil, fmt.Errorf("invalid or unqualified interface name: %s", name)
	}

	// Import the package
	imp := importer.Default()
	pkg, err := imp.Import(pkgPath)
	if err != nil {
		return nil, fmt.Errorf("failed to import package %q: %w", pkgPath, err)
	}

	// Look up the type in the package's scope
	obj := pkg.Scope().Lookup(typeName)
	if obj == nil {
		return nil, fmt.Errorf("type %q not found in package %q", typeName, pkgPath)
	}

	typeObj, ok := obj.(*types.TypeName)
	if !ok {
		return nil, fmt.Errorf("object %s.%s is not a type name but a %T", pkgPath, typeName, obj)
	}

	typ := typeObj.Type()
	if !types.IsInterface(typ) {
		return nil, fmt.Errorf("type %s is not an interface", name)
	}

	t, ok := typ.Underlying().(*types.Interface)
	if !ok {
		// This should ideally not happen if types.IsInterface passed, but check defensively.
		return nil, fmt.Errorf("type %s is an interface but failed to get underlying *types.Interface", name)
	}

	return t, nil
}

// SplitPackageAndName splits a fully qualified type name like "io.Reader" or "example.com/pkg.Type"
// into its package path and local name.
// Returns ("", "error") for built-in "error".
// Returns ("", "MyType") for unqualified "MyType".
func SplitPackageAndName(fullName string) (pkgPath string, localName string) {
	if !strings.Contains(fullName, ".") {
		// Assume built-in type (like "error") or unqualified local type.
		return "", fullName
	}
	lastDot := strings.LastIndex(fullName, ".")
	pkgPath = fullName[:lastDot]
	localName = fullName[lastDot+1:]
	return pkgPath, localName
}
