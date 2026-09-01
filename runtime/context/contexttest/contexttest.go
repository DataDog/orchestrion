// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

// Package contexttest provides test-only support for code built on top of
// github.com/DataDog/orchestrion/runtime/context.
//
// This is deliberately a separate package, rather than living directly in
// runtime/context: [MockGLS] needs per-goroutine isolation to faithfully
// simulate orchestrion's real behavior, which requires importing "runtime",
// "sync" and "strconv". runtime/context itself must not import any of
// those (see that package's registry.go and mutex.go for why), since
// orchestrion weaves a call to runtime/context into every `go` statement it
// compiles, including ones inside those very packages' own transitive
// dependencies -- so this package must never be imported by anything on
// that path, only by tests.
package contexttest

import (
	"runtime"
	"strconv"
	"sync"

	orchestrionctx "github.com/DataDog/orchestrion/runtime/context"
)

// MockGLS enables runtime/context's propagation machinery for the duration
// of a test, without requiring the test binary to have been built with
// orchestrion. It gives each goroutine its own independent context blob,
// keyed by goroutine ID, simulating the per-runtime.g storage orchestrion
// installs in a real build. It returns a cleanup function that restores the
// previous state.
//
// This is intended for use by tests of code that calls
// [orchestrionctx.Register] and exercises Controller/WrapGoroutine/
// Bootstrap/Chan behavior.
//
// Tests using MockGLS must not run in parallel with each other, as it
// mutates runtime/context's package-level state without synchronization.
func MockGLS() (cleanup func()) {
	var stacks sync.Map // goroutine ID -> any
	return orchestrionctx.SetGLSForTesting(
		func() any {
			val, _ := stacks.Load(goroutineID())
			return val
		},
		func(v any) {
			stacks.Store(goroutineID(), v)
		},
	)
}

// goroutineID extracts the current goroutine's ID from runtime.Stack
// output. This is intentionally a test-only technique: parsing goroutine
// IDs out of debug output is not something production code should rely on.
func goroutineID() uint64 {
	var buf [64]byte
	n := runtime.Stack(buf[:], false)
	s := buf[len("goroutine "):n]
	for i, b := range s {
		if b == ' ' {
			s = s[:i]
			break
		}
	}
	id, _ := strconv.ParseUint(string(s), 10, 64)
	return id
}
