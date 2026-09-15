// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package context

import "testing"

// registerSomeHooks populates the registry with a handful of entries, the
// way a real program's init-time [Register] calls would, so the benchmarks
// below measure realistic (non-empty) registry traversal costs.
func registerSomeHooks() {
	Register[string](stringHooks{})
	Register[int](intHooks{})
	Register[float64](float64Hooks{})
}

type float64Hooks struct{}

func (float64Hooks) Main() *Stack[float64]                                            { return new(Stack[float64]) }
func (float64Hooks) Go(parent *Stack[float64]) *Stack[float64]                        { return parent }
func (float64Hooks) ChanSend(parent *Stack[float64]) *Stack[float64]                  { return parent }
func (float64Hooks) ChanRecv(_ *Stack[float64], sent *Stack[float64]) *Stack[float64] { return sent }

// BenchmarkRegistrySnapshot measures the cost of [registrySnapshot] itself,
// which is the hot-path read this change touches: it's called from
// WrapGoroutine (woven into every `go` statement), Bootstrap, and every
// [Chan] Send/Recv.
func BenchmarkRegistrySnapshot(b *testing.B) {
	restore := mockGLS()
	defer restore()
	registerSomeHooks()

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = registrySnapshot()
	}
}

// BenchmarkWrapGoroutine measures the real per-`go`-statement overhead that
// orchestrion's weaving imposes on every goroutine spawn, since
// [WrapGoroutine] is what actually gets woven in place of a `go` statement
// and it calls registrySnapshot() internally.
func BenchmarkWrapGoroutine(b *testing.B) {
	restore := mockGLS()
	defer restore()
	registerSomeHooks()

	noop := func() {}

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		WrapGoroutine(noop)()
	}
}
