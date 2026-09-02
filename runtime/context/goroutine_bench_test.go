// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package context

import (
	"sync"
	"testing"
)

// registerSomeHooksOnce guards registerSomeHooks (defined in
// registry_test.go) so that BenchmarkGoWrappedEnabled can call it on every
// invocation without appending duplicate registry entries when the
// benchmark function itself runs more than once in the same process (e.g.
// under `go test -bench=. -count=2`).
var registerSomeHooksOnce sync.Once

// ensureHooksRegistered registers the shared benchmark hook set (see
// [registerSomeHooks]) exactly once per process, regardless of how many
// times the calling benchmark is invoked.
func ensureHooksRegistered() {
	registerSomeHooksOnce.Do(registerSomeHooks)
}

// BenchmarkGoBaseline measures a plain, unwrapped `go` statement: the
// "no orchestrion" baseline that BenchmarkGoWrappedDisabled and
// BenchmarkGoWrappedEnabled below are compared against.
func BenchmarkGoBaseline(b *testing.B) {
	var wg sync.WaitGroup

	b.ReportAllocs()
	wg.Add(b.N)
	for i := 0; i < b.N; i++ {
		go func() {
			defer wg.Done()
		}()
	}
	wg.Wait()
}

// BenchmarkGoWrappedDisabled measures the same goroutine spawn pattern as
// BenchmarkGoBaseline, but going through [WrapGoroutine] with propagation
// disabled -- i.e. exactly what orchestrion's `go` statement rewrite costs
// in a program that hasn't opted into (or doesn't need) context
// propagation. [WrapGoroutine] returns body unchanged when !enabled(), so
// this is expected to land close to the baseline.
//
// This benchmark deliberately does not call mockGLS: it must run with
// propagation off. It fails loudly instead of silently measuring the wrong
// thing if an earlier test or benchmark in this package left the mock GLS
// state enabled without restoring it.
func BenchmarkGoWrappedDisabled(b *testing.B) {
	if enabled() {
		b.Fatal("propagation must be disabled for this benchmark, but a previous test/benchmark left the mock GLS state enabled (missing restore()?)")
	}

	var wg sync.WaitGroup

	b.ReportAllocs()
	wg.Add(b.N)
	for i := 0; i < b.N; i++ {
		go WrapGoroutine(func() {
			defer wg.Done()
		})()
	}
	wg.Wait()
}

// BenchmarkGoWrappedEnabled measures the full real-world tax orchestrion's
// `go` statement rewrite imposes once context propagation is actually
// enabled: a synchronous registry snapshot and per-entry hook dispatch in
// the parent goroutine, plus blob installation in the child. It reuses
// mockGLS (api_test.go) and registerSomeHooks (registry_test.go) so the
// registered hook set and GLS mocking match the rest of this package's
// tests instead of duplicating that setup here.
func BenchmarkGoWrappedEnabled(b *testing.B) {
	defer mockGLS()()
	ensureHooksRegistered()

	var wg sync.WaitGroup

	b.ReportAllocs()
	wg.Add(b.N)
	for i := 0; i < b.N; i++ {
		go WrapGoroutine(func() {
			defer wg.Done()
		})()
	}
	wg.Wait()
}
