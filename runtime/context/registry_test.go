// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package context

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

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

func TestConcurrentRegistrationsAreNotLost(t *testing.T) {
	oldRegistry := registry.Load()
	registry.Store(nil)
	defer registry.Store(oldRegistry)

	const count = 32
	indices := make(chan int, count)
	var wg sync.WaitGroup
	wg.Add(count)
	for range count {
		go func() {
			defer wg.Done()
			indices <- Register[int](intHooks{}).slot.index
		}()
	}
	wg.Wait()
	close(indices)

	seen := make(map[int]struct{}, count)
	for index := range indices {
		seen[index] = struct{}{}
	}
	require.Len(t, seen, count)
	require.Len(t, registrySnapshot(), count)
}

// BenchmarkRegistrySnapshot measures the hot-path registry read used by
// goroutine propagation, Bootstrap, and every [Chan] Send/Recv.
func BenchmarkRegistrySnapshot(b *testing.B) {
	restore := mockGLS()
	defer restore()
	registerSomeHooks()

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = registrySnapshot()
	}
}
