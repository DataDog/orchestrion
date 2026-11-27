// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

// Package order exercises evaluation order and timing of the append-args
// rewrite: the spread expression runs once, the copy happens before the
// injected expressions, injected values run in template order, and panics
// propagate without being swallowed.
package order

import (
	"fmt"

	"example.com/appendargs/lib"
)

// source is the spread expression; it records its evaluation.
func source() []lib.Option {
	lib.Record.Event("source")
	return []lib.Option{lib.Named("x")}
}

// tag is a fixed-argument expression.
func tag() string {
	lib.Record.Event("tag")
	return "fixed"
}

// SpreadOnce is E01: the source expression runs exactly once.
func SpreadOnce() {
	lib.Sink(source()...)
}

// InjectedValues is E03: the three injected expressions each run once, in
// template order, after the source snapshot.
func InjectedValues(xs []lib.Option) {
	lib.Sink3(source()...)
}

// CopyBeforeMutation is E02: the injected expression mutates element zero of
// the caller's slice; the callee must see the pre-mutation element.
func CopyBeforeMutation(xs []lib.Option) {
	lib.MutSink(xs...)
}

// FixedAndSpread is E04 (matched forms): the fixed argument, then the spread
// expression, run once each.
func FixedAndSpread() {
	lib.SinkFixed(tag(), source()...)
}

// ReceiverFactory is E04 (matched forms): the receiver factory runs once.
func ReceiverFactory(opts []lib.Option) {
	lib.NewServer().Serve(opts...)
}

// panickingSource panics before returning; the panic must not be swallowed.
func panickingSource() []lib.Option {
	lib.Record.Event("source")
	panic("source panic")
}

// SourcePanic is E05.
func SourcePanic() {
	defer func() {
		if r := recover(); r != nil {
			lib.Record.Event(fmt.Sprint("recovered:", r))
		}
	}()
	lib.PanicSink(panickingSource()...)
}

// InjectedPanic is E06: the first injected expression runs, the second panics,
// and the third and the callee never run.
func InjectedPanic(xs []lib.Option) {
	defer func() {
		if r := recover(); r != nil {
			lib.Record.Event(fmt.Sprint("recovered:", r))
		}
	}()
	lib.PanicInjSink(source()...)
}

// DeferGo is E07: the deferred call and the goroutine call both see the
// statement-time snapshot of xs and the statement-time injected values, even
// though the caller mutates xs after the statements. The tests poll the record
// for the goroutine's result.
func DeferGo(xs []lib.Option) {
	defer lib.Sink(xs...)
	go lib.Sink3(xs...)
	xs[0] = lib.Named("after-defer")
}
