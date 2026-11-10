// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package emptyadvice

import (
	"testing"

	"example.com/appendargs/check"
	"example.com/appendargs/lib"
)

// TestEmptyAdviceNil is Z01: a nil spread stays nil at the callee.
func TestEmptyAdviceNil(t *testing.T) {
	lib.Record.Reset()
	NilSpread()
	check.LastHeader(t, lib.Header{Data: nil, Len: 0, Cap: 0})
	check.LastNames(t) // no elements
}

// TestEmptyAdviceEmpty is Z02: the caller's header and backing behavior are
// retained.
func TestEmptyAdviceEmpty(t *testing.T) {
	lib.Record.Reset()
	xs := make([]lib.Option, 0, 5)
	EmptySpread(xs)
	check.LastHeader(t, lib.Header{
		Data: IdentityHeader(xs).Data, Len: 0, Cap: 5,
	})
}

// TestEmptyAdviceDirect is Z03: non-spread calls stay untouched.
func TestEmptyAdviceDirect(t *testing.T) {
	lib.Record.Reset()
	DirectZero()
	check.LastNames(t)

	lib.Record.Reset()
	DirectOne(lib.Named("x"))
	check.LastNames(t, "x")
}

// TestEmptyAdviceTuple is Z04: a sole tuple call stays accepted.
func TestEmptyAdviceTuple(t *testing.T) {
	lib.Record.Reset()
	TupleSpread()
	check.LastNames(t, "a", "b")
}

// TestEmptyAdviceControl is the positive control: the module's weaving is
// live, so the lib.Sink call in Control gets its injected value. The no-op
// assertions above therefore test a woven package, not a dead config.
func TestEmptyAdviceControl(t *testing.T) {
	lib.Record.Reset()
	Control([]lib.Option{lib.Named("x")})
	check.LastNames(t, "x", "traced")
}

// TestEmptyAdviceIdentity is Z06: with spare capacity, the callee receives the
// same data pointer, length, and capacity as the caller.
func TestEmptyAdviceIdentity(t *testing.T) {
	lib.Record.Reset()
	full := make([]lib.Option, 5)
	xs := full[:2]
	want := IdentityHeader(xs)
	Identity(xs)
	check.LastHeader(t, want)
}
