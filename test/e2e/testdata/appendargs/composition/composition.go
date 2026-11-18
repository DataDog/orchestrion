// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

// Package composition exercises append-args combined with itself and with
// other advice on the same call.
package composition

import "example.com/appendargs/lib"

// TwoAspects is C10: two append-args aspects apply to the same spread call.
// The first aspect's injected expression mutates the caller's slice after its
// snapshot; the second aspect's snapshot includes the first aspect's value.
func TwoAspects(xs []lib.Option) {
	lib.Composed(xs...)
}

// EmptyAspect is C12: one aspect has zero templates (a no-op) and the other
// adds a value.
func EmptyAspect(xs []lib.Option) {
	lib.ComposedEmpty(xs...)
}

// OtherAdvice is C14: append-args combined with replace-function.
func OtherAdvice(xs []lib.Option) []lib.Option {
	return lib.Redirect(xs...)
}

// OtherAdviceReversed is C14 in the reverse advice order.
func OtherAdviceReversed(xs []lib.Option) []lib.Option {
	return lib.RedirectRev(xs...)
}

// Wrapped is C14 with wrap-expression first: ReplaceNode moves the advised
// node, so the injected value lands in the variadic tail of the wrapper.
func Wrapped(xs []lib.Option) []lib.Option {
	return lib.WrapSink(xs...)
}

// WrappedReversed is C14 with append-args first: the spread call is rewritten
// before the wrap moves it into the wrapper.
func WrappedReversed(xs []lib.Option) []lib.Option {
	return lib.WrapSinkRev(xs...)
}
