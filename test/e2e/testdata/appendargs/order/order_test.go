// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package order

import (
	"testing"
	"time"

	"example.com/appendargs/check"
	"example.com/appendargs/lib"
)

// waitForCalls polls until the record holds n calls, and fails after a timeout.
func waitForCalls(t *testing.T, n int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for len(lib.Record.Names()) < n && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if len(lib.Record.Names()) < n {
		t.Fatalf("only %d calls recorded, want %d", len(lib.Record.Names()), n)
	}
}

// TestOrderSpreadOnce is E01: the spread expression runs once before the
// injected call, and the callee sees the original element.
func TestOrderSpreadOnce(t *testing.T) {
	lib.Record.Reset()
	SpreadOnce()
	check.Events(t, "source")
	check.Count(t, "one", 0)
	check.LastNames(t, "x", "traced")
}

// TestOrderInjectedValues is E03: every injected expression runs once, in
// template order, after the source snapshot.
func TestOrderInjectedValues(t *testing.T) {
	lib.Record.Reset()
	InjectedValues(nil)
	check.Events(t, "source", "one", "two", "three")
	check.Count(t, "one", 1)
	check.Count(t, "two", 1)
	check.Count(t, "three", 1)
	check.LastNames(t, "x", "one", "two", "three")
}

// TestOrderCopyBeforeMutation is E02: the clone completes before the injected
// expression mutates the source, so the callee sees the pre-mutation element.
// The source mutation itself remains visible afterwards.
func TestOrderCopyBeforeMutation(t *testing.T) {
	lib.Record.Reset()
	xs := []lib.Option{lib.Named("x")}
	CopyBeforeMutation(xs)
	check.LastNames(t, "x", "traced")
	if xs[0].Name != "mutated" {
		t.Fatalf("source mutation not visible afterwards: %v", xs)
	}
}

// TestOrderFunctionReceiverAndFixed is E04: the fixed argument, then the spread
// expression, and the receiver factory each run exactly once.
func TestOrderFunctionReceiverAndFixed(t *testing.T) {
	lib.Record.Reset()
	FixedAndSpread()
	check.Events(t, "tag", "source")
	check.LastNames(t, "x", "traced")

	lib.Record.Reset()
	ReceiverFactory([]lib.Option{lib.Named("x")})
	check.Events(t, "new-server")
	check.LastNames(t, "x", "traced")
}

// TestOrderSourcePanic is E05: the source panic is not swallowed, and neither
// the injected expression nor the callee runs.
func TestOrderSourcePanic(t *testing.T) {
	lib.Record.Reset()
	SourcePanic()
	check.Events(t, "source", "recovered:source panic")
	check.Count(t, "one", 0)
	check.CallCount(t, 0)
}

// TestOrderInjectedPanic is E06: the first injected expression runs once, the
// panicking one propagates, and the later injected expression and the callee
// never run.
func TestOrderInjectedPanic(t *testing.T) {
	lib.Record.Reset()
	InjectedPanic(nil)
	check.Events(t, "source", "one", "recovered:injected panic")
	check.Count(t, "one", 1)
	check.Count(t, "three", 0)
	check.CallCount(t, 0)
}

// TestOrderDeferGo is E07: deferred and goroutine calls see the statement-time
// snapshot and the statement-time injected values.
func TestOrderDeferGo(t *testing.T) {
	lib.Record.Reset()
	DeferGo([]lib.Option{lib.Named("x")})
	waitForCalls(t, 2)

	calls := lib.Record.Names()
	if len(calls) != 2 {
		t.Fatalf("recorded %v", calls)
	}
	for _, got := range calls {
		if len(got) == 0 || got[0] != "x" {
			t.Fatalf("statement-time snapshot lost: %v", got)
		}
		if got[len(got)-1] != "traced" && got[len(got)-1] != "three" {
			t.Fatalf("statement-time injected values lost: %v", got)
		}
	}
	// The deferred call gets its single injected value; the goroutine call gets
	// its three, both evaluated at the statements.
	if len(calls[0]) != 2 || len(calls[1]) != 4 {
		t.Fatalf("recorded %v", calls)
	}
}
