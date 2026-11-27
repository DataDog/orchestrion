// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

// Package emptyadvice exercises zero-template advice: a true no-op that
// leaves the call, its imports, and the slice identity untouched.
package emptyadvice

import (
	"unsafe"

	"example.com/appendargs/lib"
)

func NilSpread() {
	lib.NoopSink(nil...)
}

func EmptySpread(xs []lib.Option) {
	lib.NoopSink(xs...)
}

func DirectZero() {
	lib.NoopSink()
}

func DirectOne(x lib.Option) {
	lib.NoopSink(x)
}

func TupleSpread() {
	lib.NoopSink(lib.TwoOpts())
}

// Control is the positive control for the zero-template no-op: the lib.Sink
// aspect is live in this module, so this call weaves and its value appears.
// It proves the no-op assertions above test a woven package, not a dead config.
func Control(xs []lib.Option) {
	lib.Sink(xs...)
}

// Identity is Z06: a non-empty slice with spare capacity; the callee must
// receive the same slice header as the caller.
func Identity(xs []lib.Option) {
	lib.NoopSink(xs...)
}

// IdentityHeader exposes the caller-side header for the assertions.
func IdentityHeader(xs []lib.Option) lib.Header {
	return lib.Header{Data: unsafe.Pointer(unsafe.SliceData(xs)), Len: len(xs), Cap: cap(xs)}
}
