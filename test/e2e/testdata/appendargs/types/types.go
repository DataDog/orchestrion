// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

// Package types exercises generic callers: an in-scope type parameter and a
// type-parameter slice, both through inferred generic calls.
package types

import "example.com/appendargs/lib"

// Generic is T02: the caller is generic, and the configured type is its
// in-scope type parameter T; the injected value is a T-typed argument.
func Generic[T any](vals []T, extra T) {
	lib.GenericSink(vals...)
}

// TypeParamSlice is T08: the spread argument is a type-parameter slice
// S (~[]E), and the configured type is E.
func TypeParamSlice[S ~[]E, E any](xs S, extra E) {
	lib.GenericSinkE(xs...)
}
