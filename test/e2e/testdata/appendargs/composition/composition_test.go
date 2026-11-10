// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package composition

import (
	"testing"

	"example.com/appendargs/check"
	"example.com/appendargs/lib"
)

// TestCompositionTwoAspects is C10: the original source runs once, the inner
// snapshot completes before the first aspect's value, the outer snapshot
// before the second value, and each value appears once.
func TestCompositionTwoAspects(t *testing.T) {
	lib.Record.Reset()
	xs := []lib.Option{lib.Named("x")}
	TwoAspects(xs)
	check.LastNames(t, "x", "traced", "second")
	// The first aspect's value mutates the caller's slice after its snapshot,
	// and that mutation stays visible to the caller only.
	if xs[0].Name != "mutated" {
		t.Fatalf("source mutation not visible afterwards: %v", xs)
	}
}

// TestCompositionEmptyAspect is C12: the empty aspect leaves the call, its
// imports, and its ordering alone; the non-empty aspect adds exactly one
// value.
func TestCompositionEmptyAspect(t *testing.T) {
	lib.Record.Reset()
	EmptyAspect([]lib.Option{lib.Named("x")})
	check.LastNames(t, "x", "traced")
}

// TestCompositionOtherAdvice is C14: each value appears once through the
// replace-function combination, in both advice orders.
func TestCompositionOtherAdvice(t *testing.T) {
	lib.Record.Reset()
	got := OtherAdvice([]lib.Option{lib.Named("x")})
	check.LastNames(t, "x", "traced")
	if len(got) != 2 || got[0].Name != "x" || got[1].Name != "traced" {
		t.Fatalf("returned %v", got)
	}

	lib.Record.Reset()
	got = OtherAdviceReversed([]lib.Option{lib.Named("x")})
	check.LastNames(t, "x", "traced")
	if len(got) != 2 || got[0].Name != "x" || got[1].Name != "traced" {
		t.Fatalf("returned %v", got)
	}
}

// TestCompositionWrapped is C14: append-args combined with wrap-expression
// keeps every value exactly once. With the wrap first, ReplaceNode moves the
// advised node, so the value lands in the wrapper's variadic tail: the sink
// sees the original values and the wrapper sees the originals plus the value.
// With append-args first, the spread call is rewritten before the wrap moves
// it, so both the sink and the wrapper see the value.
func TestCompositionWrapped(t *testing.T) {
	lib.Record.Reset()
	got := Wrapped([]lib.Option{lib.Named("x")})
	names := lib.Record.Names()
	if len(names) != 2 {
		t.Fatalf("recorded %v", names)
	}
	if len(names[0]) != 1 || names[0][0] != "x" {
		t.Fatalf("sink recorded %v, want [x]", names[0])
	}
	if len(names[1]) != 2 || names[1][0] != "x" || names[1][1] != "traced" {
		t.Fatalf("wrapper recorded %v, want [x traced]", names[1])
	}
	if len(got) != 2 || got[0].Name != "x" || got[1].Name != "traced" {
		t.Fatalf("returned %v", got)
	}

	lib.Record.Reset()
	got = WrappedReversed([]lib.Option{lib.Named("x")})
	names = lib.Record.Names()
	if len(names) != 2 {
		t.Fatalf("recorded %v", names)
	}
	for i, recorded := range names {
		if len(recorded) != 2 || recorded[0] != "x" || recorded[1] != "traced" {
			t.Fatalf("recording %d = %v, want [x traced]", i, recorded)
		}
	}
	if len(got) != 2 || got[0].Name != "x" || got[1].Name != "traced" {
		t.Fatalf("returned %v", got)
	}
}
