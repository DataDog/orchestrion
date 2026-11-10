// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

// Package local holds the sink and the value type for the go1.17 language
// fixture. It deliberately uses nothing newer than the go1.17 language.
package local

import "sync"

type Opt struct{ Name string }

var (
	mu    sync.Mutex
	calls [][]string
)

// Record returns the option names recorded by the sink calls.
func Record() [][]string {
	mu.Lock()
	defer mu.Unlock()
	out := make([][]string, len(calls))
	copy(out, calls)
	return out
}

// Reset clears the record.
func Reset() {
	mu.Lock()
	defer mu.Unlock()
	calls = nil
}

func Sink(opts ...Opt) {
	mu.Lock()
	defer mu.Unlock()
	names := make([]string, len(opts))
	for i, opt := range opts {
		names[i] = opt.Name
	}
	calls = append(calls, names)
}

// SinkZero is the callee of the zero-template advice: the advice is a no-op,
// so the package stays valid at the go1.17 language level.
func SinkZero(opts ...Opt) {
	Sink(opts...)
}

func Named(name string) Opt { return Opt{Name: name} }
