// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package context

import (
	"sync"
	"testing"
)

var registerSomeHooksOnce sync.Once

func ensureHooksRegistered() {
	registerSomeHooksOnce.Do(registerSomeHooks)
}

// BenchmarkGoBaseline records native goroutine creation cost. The woven
// no-registration path adds only a nil atomic-pointer check in runtime.newproc.
func BenchmarkGoBaseline(b *testing.B) {
	var wg sync.WaitGroup
	b.ReportAllocs()
	wg.Add(b.N)
	for i := 0; i < b.N; i++ {
		go func() { wg.Done() }()
	}
	wg.Wait()
}

// BenchmarkPropagateGoroutine measures the parent-side callback work that the
// instrumented runtime performs when hooks are registered. Child creation and
// scheduling are deliberately excluded so hook dispatch is visible directly.
func BenchmarkPropagateGoroutine(b *testing.B) {
	restore := mockGLS()
	defer restore()
	ensureHooksRegistered()
	parent := getBlob()

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = propagateGoroutine(parent)
	}
}
