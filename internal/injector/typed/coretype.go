// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package typed

import "go/types"

// maxCoreTypeDepth bounds the recursion performed while computing the core type
// of a type parameter, so that pathological (or cyclic) constraint hierarchies
// can never cause unbounded recursion.
const maxCoreTypeDepth = 16

// CoreType returns the core type of t as defined by the Go language
// specification, or nil if t has no core type (for example an interface with
// more than one distinct underlying type in its type set).
//
// For ordinary (non type-parameter) types, the core type is simply the
// underlying type. For a type parameter, it is the single underlying type shared
// by every term of its constraint's type set.
func CoreType(t types.Type) types.Type {
	if t == nil {
		return nil
	}

	t = types.Unalias(t)
	if tp, ok := t.(*types.TypeParam); ok {
		return typeParamCoreType(tp, maxCoreTypeDepth)
	}

	return t.Underlying()
}

// typeParamCoreType computes the core type of a type parameter by intersecting
// the underlying types of all the terms in its constraint's type set. It returns
// nil when the type set is empty, unbounded, or contains terms with differing
// underlying types.
func typeParamCoreType(tp *types.TypeParam, depth int) types.Type {
	iface, ok := types.Unalias(tp.Constraint()).Underlying().(*types.Interface)
	if !ok {
		return nil
	}

	core, found := interfaceCoreType(iface, depth)
	if !found {
		return nil
	}
	return core
}

// interfaceCoreType returns the single underlying type shared by every term of
// the given interface's type set. The boolean result reports whether such a type
// was found; it is false when the interface constrains no types at all (only
// methods), or when its terms do not all share the same underlying type.
func interfaceCoreType(iface *types.Interface, depth int) (types.Type, bool) {
	if depth <= 0 {
		return nil, false
	}

	var core types.Type
	for i := range iface.NumEmbeddeds() {
		embedded := types.Unalias(iface.EmbeddedType(i))
		if named, ok := embedded.(*types.Named); ok {
			embedded = named.Underlying()
		}

		var terms []types.Type
		switch embedded := embedded.(type) {
		case *types.Union:
			for j := range embedded.Len() {
				terms = append(terms, types.Unalias(embedded.Term(j).Type()))
			}
		case *types.Interface:
			// Method-only and comparable interfaces do not widen an intersection,
			// so they contribute no core term. A nested type restriction contributes
			// its own core type.
			nested, found := interfaceCoreType(embedded, depth-1)
			if !found {
				continue
			}
			terms = []types.Type{nested}
		default:
			terms = []types.Type{embedded}
		}

		for _, term := range terms {
			underlying := term.Underlying()
			if core == nil {
				core = underlying
				continue
			}
			if !types.Identical(core, underlying) {
				// More than one distinct underlying type: there is no core type.
				return nil, false
			}
		}
	}

	return core, core != nil
}

// IsStringCore reports whether the core type of t is exactly the predeclared
// `string` type. This is true of `string` itself, of defined types whose
// underlying type is `string`, and of type parameters constrained to a single
// `string`-based type set (such as `~string`).
//
// Untyped string constants are not matched, as their type is
// [types.UntypedString].
func IsStringCore(t types.Type) bool {
	basic, ok := CoreType(t).(*types.Basic)
	return ok && basic.Kind() == types.String
}

// IsByteSliceCore reports whether the core type of t is exactly `[]byte`. Slices
// of a defined type whose underlying type is `byte` (such as
// `type myByte byte`) are deliberately not matched, as they are not `[]byte`.
func IsByteSliceCore(t types.Type) bool {
	slice, ok := CoreType(t).(*types.Slice)
	return ok && types.Identical(slice.Elem(), types.Typ[types.Byte])
}
