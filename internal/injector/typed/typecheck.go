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
	pkgPath, typeName := SplitPackageAndName(name)

	if pkgPath == "" {
		// Handle built-in types or unqualified names.
		scope := types.Universe
		obj := scope.Lookup(typeName)
		if obj == nil {
			// Not found in universe scope.
			return nil, fmt.Errorf("interface %q not found (not a built-in or unqualified)", typeName)
		}
		// Found in universe, now validate it's an interface type name.
		return validateTypeNameIsInterface(obj, name, pkgPath, typeName)
	}

	// Handle package-qualified types (e.g., "io.Writer").
	imp := importer.Default()
	pkg, err := imp.Import(pkgPath)
	if err != nil {
		// Specific error for import failure.
		return nil, fmt.Errorf("failed to import package %q: %w", pkgPath, err)
	}

	scope := pkg.Scope()
	obj := scope.Lookup(typeName)
	if obj == nil {
		// Not found within the imported package's scope.
		return nil, fmt.Errorf("type %q not found in package %q", typeName, pkgPath)
	}

	// Found in package scope, now validate it's an interface type name.
	return validateTypeNameIsInterface(obj, name, pkgPath, typeName)
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

// validateTypeNameIsInterface checks if a successfully looked-up types.Object represents
// a type name that resolves to an interface. It assumes obj is not nil.
func validateTypeNameIsInterface(obj types.Object, fullName string, pkgPath string, typeName string) (*types.Interface, error) {
	typeObj, ok := obj.(*types.TypeName)
	if !ok {
		// Provide context whether it was expected to be built-in or package-qualified.
		if pkgPath == "" {
			return nil, fmt.Errorf("object %q is not a type name but a %T", typeName, obj)
		}
		return nil, fmt.Errorf("object %s.%s is not a type name but a %T", pkgPath, typeName, obj)
	}

	typ := typeObj.Type()
	if !types.IsInterface(typ) {
		// Use the original full name in the error message for clarity.
		return nil, fmt.Errorf("type %s is not an interface", fullName)
	}

	// Since types.IsInterface passed, we can safely cast typ.Underlying() to *types.Interface
	t, ok := typ.Underlying().(*types.Interface)
	if !ok {
		return nil, fmt.Errorf("type %s is not an interface", fullName)
	}
	return t, nil
}
