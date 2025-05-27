// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package typed

import "fmt"

// ExtractNamedType unwraps pointer types to extract the underlying NamedType.
// It follows pointer chains until it finds a non-pointer type.
// Returns an error if the final type is not a NamedType.
func ExtractNamedType(t Type) (*NamedType, error) {
	// Unwrap pointer types
	for ptr, ok := t.(*PointerType); ok; ptr, ok = t.(*PointerType) {
		t = ptr.Elem
	}

	// Check if the final type is a NamedType
	nt, ok := t.(*NamedType)
	if !ok {
		return nil, fmt.Errorf("expected a named type or pointer to named type, got %T", t)
	}

	return nt, nil
}
