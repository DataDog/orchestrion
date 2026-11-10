// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

// Package callableappend declares a package-scope function named append with a
// compatible signature and an observable counter. The rewrite must never call
// it, even where calling it would compile.
package callableappend

import (
	"sync/atomic"

	"example.com/appendargs/lib"
)

var appendCalls atomic.Int32

// append shadows the builtin for this whole package, with a signature the
// historical closure would have accepted.
func append(opts ...lib.Option) []lib.Option {
	appendCalls.Add(1)
	return opts
}

// CallableAppend spreads into the woven sink with the shadowing name in scope.
func CallableAppend(args []lib.Option) {
	lib.Sink(args...)
}

// AppendCalls returns how many times the caller's append ran.
func AppendCalls() int { return int(appendCalls.Load()) }
