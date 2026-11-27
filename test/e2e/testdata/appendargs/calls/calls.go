// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

// Package calls exercises the call-form rows: non-spread calls, fixed and
// spread combinations, method calls, function-value calls, and a
// package-level initializer.
package calls

import (
	"example.com/appendargs/lib"
)

// Zero is C01: a non-spread call with zero original arguments.
func Zero() {
	lib.Sink()
}

// One is C02: a non-spread call with one original argument, including a
// one-result call.
func One(x lib.Option) {
	lib.Sink(x)
}

func OneResult(x lib.Option) {
	lib.Sink(oneResult(x))
}

func oneResult(x lib.Option) lib.Option {
	return x
}

// Many is C03.
func Many(x, y, z lib.Option) {
	lib.Sink(x, y, z)
}

// FixedAndSpread is C04: fixed arguments before the spread; only the spread
// argument is transformed.
func FixedAndSpread(tag string, args []lib.Option) {
	lib.SinkFixed(tag, args...)
}

// tag is a fixed-argument expression with an observable effect.
func tag() string {
	lib.Record.Event("tag")
	return "fixed"
}

// TaggedSource combines a fixed argument call with the spread.
func TaggedSource(args []lib.Option) {
	lib.SinkFixed(tag(), args...)
}

// MethodValue is C08: a woven method call on a value receiver, with caller
// opts and append names in scope.
func MethodValue(opts []lib.Option) {
	append := 1
	_ = append
	srv := lib.Server{}
	srv.Serve(opts...)
}

// MethodPointer is C08: the same method through a pointer receiver.
func MethodPointer(p *lib.Server, opts []lib.Option) {
	append := 1
	_ = append
	p.Serve(opts...)
}

// MethodFactory is C08: the receiver is itself produced by a factory call that
// must run once.
func MethodFactory(opts []lib.Option) {
	lib.NewServer().Serve(opts...)
}

// sinkFn is a function value; the syntactic function-call join point on its
// name selects the call for C09.
var sinkFn = lib.Sink

// FunctionValue is C09: the call goes through a function value.
func FunctionValue(args []lib.Option) {
	sinkFn(args...)
}

// argsPkg feeds the package-level initializer below.
var argsPkg = []lib.Option{lib.Named("init-x")}

// V is E08: a package-level initializer holds the woven spread call, and the
// injected value reads a variable of another package.
var V = lib.PkgInitSink(argsPkg...)
