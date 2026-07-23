// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package join

import "go/types"

func isString(value types.Type) bool {
	if value == nil {
		return false
	}
	if parameter, ok := value.(*types.TypeParam); ok {
		return constraintOnly(parameter.Constraint(), isString)
	}
	basic, ok := value.Underlying().(*types.Basic)
	return ok && basic.Kind() == types.String
}

func isByteSlice(value types.Type) bool {
	if value == nil {
		return false
	}
	if parameter, ok := value.(*types.TypeParam); ok {
		return constraintOnly(parameter.Constraint(), isByteSlice)
	}
	slice, ok := value.Underlying().(*types.Slice)
	return ok && isByte(slice.Elem())
}

func isByte(value types.Type) bool {
	if value == nil {
		return false
	}
	if parameter, ok := value.(*types.TypeParam); ok {
		return constraintOnly(parameter.Constraint(), isByte)
	}
	basic, ok := value.Underlying().(*types.Basic)
	return ok && basic.Kind() == types.Uint8
}

func isRuneSlice(value types.Type) bool {
	if value == nil {
		return false
	}
	if parameter, ok := value.(*types.TypeParam); ok {
		return constraintOnly(parameter.Constraint(), isRuneSlice)
	}
	slice, ok := value.Underlying().(*types.Slice)
	return ok && isRune(slice.Elem())
}

func isRune(value types.Type) bool {
	if value == nil {
		return false
	}
	if parameter, ok := value.(*types.TypeParam); ok {
		return constraintOnly(parameter.Constraint(), isRune)
	}
	basic, ok := value.Underlying().(*types.Basic)
	return ok && basic.Kind() == types.Int32
}

func constraintOnly(constraint types.Type, predicate func(types.Type) bool) bool {
	iface, ok := constraint.Underlying().(*types.Interface)
	if !ok {
		return false
	}
	iface.Complete()
	found := false
	for index := range iface.NumEmbeddeds() {
		union, ok := iface.EmbeddedType(index).(*types.Union)
		if !ok {
			continue
		}
		for termIndex := range union.Len() {
			found = true
			if !predicate(union.Term(termIndex).Type()) {
				return false
			}
		}
	}
	return found
}
