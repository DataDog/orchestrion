// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

// Package slices exercises the slice-value behavior of the append-args
// rewrite: nilness, backing-array protection, spare capacity, shared backing,
// and element-type shapes.
package slices

import (
	"sync"

	"example.com/appendargs/lib"
)

func NilOne()  { lib.Sink(nil...) }
func NilMany() { lib.Sink3(nil...) }

func EmptyOne(xs []lib.Option)  { lib.Sink(xs...) }
func EmptyMany(xs []lib.Option) { lib.Sink3(xs...) }

func FullOne(xs []lib.Option) { lib.Sink(xs...) }

// SpareOne receives a slice with spare capacity and sentinel elements beyond
// its length; the callee gets a copy and must not touch the caller's storage.
func SpareOne(xs []lib.Option) { lib.Sink(xs...) }

// Alternate spreads two overlapping subslices that share backing storage.
func Alternate(a, b []lib.Option) {
	lib.Sink(a...)
	lib.Sink(b...)
}

// CalleeWrites lets the callee assign to its variadic elements; the writes
// must land in the callee's own slice.
func CalleeWrites(xs []lib.Option) { lib.SinkCalleeWrites(xs...) }

// ShallowCopy checks that pointer elements keep their identity through the
// copy.
func ShallowCopy(vals []*lib.Box) { lib.PtrSink(vals...) }

// Concurrent runs many woven calls that read the same spare-capacity slice.
// The caller slice is only ever read, and the injected values are
// independent, so the calls must be race-free.
func Concurrent(xs []lib.Option, rounds int) {
	var wg sync.WaitGroup
	for range rounds {
		wg.Add(1)
		go func() {
			defer wg.Done()
			lib.Sink(xs...)
		}()
	}
	wg.Wait()
}

func LiteralNil()                      { lib.Sink(nil...) }
func LiteralEmpty()                    { lib.Sink([]lib.Option{}...) }
func LiteralPopulated(a, b lib.Option) { lib.Sink([]lib.Option{a, b}...) }

// SpreadDefined and SpreadAlias spread a defined slice type and a slice alias.
func SpreadDefined(opts lib.Options) { lib.Sink(opts...) }
func SpreadAlias(opts lib.OptAlias)  { lib.Sink(opts...) }

// InterfaceMixed spreads mixed concrete values and a typed-nil pointer into
// the any-typed sink.
func InterfaceMixed(vals []any) { lib.AnySink(vals...) }

// Pointers spreads pointer values, including nil elements.
func Pointers(vals []*lib.Box) { lib.PtrSink(vals...) }
