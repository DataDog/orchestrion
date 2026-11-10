// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

// Package check provides the small assertion helpers the append-args fixture
// packages share. The fixture modules deliberately depend on nothing outside
// the standard library, so they cannot use testify, and they never import
// slices: the rewrite is what introduces that import (V08).
package check

import (
	"testing"

	"example.com/appendargs/lib"
)

func eqStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

// LastNames asserts the option names recorded by the most recent sink call.
func LastNames(t *testing.T, want ...string) {
	t.Helper()
	got := lib.Record.LastNames()
	if !eqStrings(got, want) {
		t.Fatalf("recorded names %v, want %v", got, want)
	}
}

// Count asserts the number of recorded effects with the given label.
func Count(t *testing.T, label string, want int) {
	t.Helper()
	if got := lib.Record.Count(label); got != want {
		t.Fatalf("count(%q) = %d, want %d", label, got, want)
	}
}

// Events asserts the full ordered event log.
func Events(t *testing.T, want ...string) {
	t.Helper()
	got := lib.Record.Events()
	if !eqStrings(got, want) {
		t.Fatalf("events %v, want %v", got, want)
	}
}

// LastHeader asserts the slice header the most recent option-valued sink
// observed.
func LastHeader(t *testing.T, want lib.Header) {
	t.Helper()
	got := lib.Record.LastHeader()
	if got != want {
		t.Fatalf("header %+v, want %+v", got, want)
	}
}

// CallCount asserts how many sink calls were recorded.
func CallCount(t *testing.T, want int) {
	t.Helper()
	if got := len(lib.Record.Names()); got != want {
		t.Fatalf("recorded %d calls, want %d", got, want)
	}
}

// True asserts cond with the given message.
func True(t *testing.T, cond bool, msg string) {
	t.Helper()
	if !cond {
		t.Fatal(msg)
	}
}
