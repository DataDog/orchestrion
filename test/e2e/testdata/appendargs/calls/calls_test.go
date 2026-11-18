// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package calls

import (
	"testing"

	"example.com/appendargs/check"
	"example.com/appendargs/lib"
)

// TestCallsZero is C01.
func TestCallsZero(t *testing.T) {
	lib.Record.Reset()
	Zero()
	check.LastNames(t, "traced")
}

// TestCallsOne is C02: one original argument, plain and one-result call.
func TestCallsOne(t *testing.T) {
	lib.Record.Reset()
	One(lib.Named("x"))
	check.LastNames(t, "x", "traced")

	lib.Record.Reset()
	OneResult(lib.Named("y"))
	check.LastNames(t, "y", "traced")
}

// TestCallsMany is C03.
func TestCallsMany(t *testing.T) {
	lib.Record.Reset()
	Many(lib.Named("x"), lib.Named("y"), lib.Named("z"))
	check.LastNames(t, "x", "y", "z", "traced")
}

// TestCallsFixedAndSpread is C04: fixed arguments are preserved and only the
// spread argument is transformed.
func TestCallsFixedAndSpread(t *testing.T) {
	lib.Record.Reset()
	FixedAndSpread("fixed", []lib.Option{lib.Named("x")})
	check.LastNames(t, "x", "traced")

	lib.Record.Reset()
	TaggedSource([]lib.Option{lib.Named("x")})
	check.Events(t, "tag")
	check.LastNames(t, "x", "traced")
}

// TestCallsMethod is C08: value and pointer receiver calls with caller opts and
// append names, and a receiver factory that runs once.
func TestCallsMethod(t *testing.T) {
	lib.Record.Reset()
	MethodValue([]lib.Option{lib.Named("x")})
	check.LastNames(t, "x", "traced")

	lib.Record.Reset()
	MethodPointer(&lib.Server{}, []lib.Option{lib.Named("x")})
	check.LastNames(t, "x", "traced")

	lib.Record.Reset()
	MethodFactory([]lib.Option{lib.Named("x")})
	check.Events(t, "new-server")
	check.LastNames(t, "x", "traced")
}

// TestCallsFunctionValue is C09: the call through the function value is
// selected by the syntactic join point and transformed like any other.
func TestCallsFunctionValue(t *testing.T) {
	lib.Record.Reset()
	FunctionValue([]lib.Option{lib.Named("x")})
	check.LastNames(t, "x", "traced")
}

// TestCallsPackageInit is E08: the package initializer's woven call recorded
// the package variables plus the injected value read from another package,
// and the initializer result is correct.
func TestCallsPackageInit(t *testing.T) {
	if V != 2 {
		t.Fatalf("package initializer result = %d, want 2", V)
	}
}
