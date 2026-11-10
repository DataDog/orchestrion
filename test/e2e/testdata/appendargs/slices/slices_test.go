// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package slices

import (
	"testing"

	"example.com/appendargs/check"
	"example.com/appendargs/lib"
)

// TestSlicesNil is S01: with one template, a nil input reaches the callee as a
// non-nil one-element slice.
func TestSlicesNil(t *testing.T) {
	lib.Record.Reset()
	NilOne()
	names := lib.Record.LastNames()
	if len(names) != 1 || names[0] != "traced" {
		t.Fatalf("recorded %v, want [traced]", names)
	}
	hdr := lib.Record.LastHeader()
	if hdr.Data == nil {
		t.Fatal("the callee must receive a non-nil slice for a nil input")
	}
}

// TestSlicesEmpty is S02: an empty non-nil input keeps the caller's header and
// the callee sees only the injected values.
func TestSlicesEmpty(t *testing.T) {
	lib.Record.Reset()
	xs := make([]lib.Option, 0)
	EmptyOne(xs)
	check.LastNames(t, "traced")
	if len(xs) != 0 || cap(xs) != 0 {
		t.Fatalf("caller header changed: len=%d cap=%d", len(xs), cap(xs))
	}

	lib.Record.Reset()
	full := make([]lib.Option, 0, 5)
	EmptyOne(full)
	check.LastNames(t, "traced")
	if len(full) != 0 || cap(full) != 5 {
		t.Fatalf("caller header changed: len=%d cap=%d", len(full), cap(full))
	}
}

// TestSlicesFullCapacity is S03: a full-capacity input reaches the callee with
// the original elements followed by the injected one, and the source does not
// change.
func TestSlicesFullCapacity(t *testing.T) {
	lib.Record.Reset()
	xs := []lib.Option{lib.Named("a"), lib.Named("b")}
	FullOne(xs)
	check.LastNames(t, "a", "b", "traced")
	if len(xs) != 2 || cap(xs) != 2 || xs[0].Name != "a" || xs[1].Name != "b" {
		t.Fatalf("source changed: %v", xs)
	}
}

// TestSlicesSpareCapacity is S04: no injected element overwrites the sentinel
// storage beyond the caller's length.
func TestSlicesSpareCapacity(t *testing.T) {
	lib.Record.Reset()
	full := []lib.Option{
		lib.Named("a"), lib.Named("b"),
		lib.Named("s0"), lib.Named("s1"), lib.Named("s2"),
	}
	xs := full[:2]
	SpareOne(xs)
	check.LastNames(t, "a", "b", "traced")
	if len(xs) != 2 || cap(xs) != 5 {
		t.Fatalf("caller header changed: len=%d cap=%d", len(xs), cap(xs))
	}
	for i, want := range []string{"a", "b", "s0", "s1", "s2"} {
		if full[i].Name != want {
			t.Fatalf("backing element %d = %q, want %q", i, full[i].Name, want)
		}
	}
}

// TestSlicesSharedBacking is S05: alternating calls through two overlapping
// subslices change neither source view nor the hidden tail.
func TestSlicesSharedBacking(t *testing.T) {
	lib.Record.Reset()
	full := []lib.Option{
		lib.Named("a"), lib.Named("b"), lib.Named("c"),
		lib.Named("s0"), lib.Named("s1"),
	}
	a := full[:2]
	b := full[1:3]
	Alternate(a, b)
	if got := len(lib.Record.Names()); got != 2 {
		t.Fatalf("recorded %d calls, want 2", got)
	}
	want := [][]string{{"a", "b", "traced"}, {"b", "c", "traced"}}
	for i, w := range want {
		got := lib.Record.Names()[i]
		if len(got) != len(w) {
			t.Fatalf("call %d recorded %v, want %v", i, got, w)
		}
		for j := range w {
			if got[j] != w[j] {
				t.Fatalf("call %d recorded %v, want %v", i, got, w)
			}
		}
	}
	for i, wantName := range []string{"a", "b", "c", "s0", "s1"} {
		if full[i].Name != wantName {
			t.Fatalf("backing element %d = %q, want %q", i, full[i].Name, wantName)
		}
	}
}

// TestSlicesCalleeWrites is S06: the callee writes to its own variadic slice
// without changing the caller's backing elements.
func TestSlicesCalleeWrites(t *testing.T) {
	lib.Record.Reset()
	xs := []lib.Option{lib.Named("a"), lib.Named("b")}
	CalleeWrites(xs)
	// The callee overwrote its first element before recording.
	check.LastNames(t, "clobbered", "b", "traced")
	if xs[0].Name != "a" || xs[1].Name != "b" {
		t.Fatalf("caller elements changed: %v", xs)
	}
}

// TestSlicesShallowCopy is S07: pointer elements keep their identity through
// the copy.
func TestSlicesShallowCopy(t *testing.T) {
	lib.Record.Reset()
	b := &lib.Box{Name: "b"}
	ShallowCopy([]*lib.Box{b, nil})
	ptrs := lib.Record.Ptrs()
	if len(ptrs) != 1 || len(ptrs[0]) != 3 {
		t.Fatalf("recorded %v", ptrs)
	}
	if ptrs[0][0] != b || ptrs[0][1] != nil {
		t.Fatalf("element identity changed: %v", ptrs[0])
	}
	if ptrs[0][2] == nil || ptrs[0][2].Name != "traced" {
		t.Fatalf("injected pointer wrong: %v", ptrs[0][2])
	}
}

// TestSlicesConcurrent is S08: concurrent woven calls over a shared
// spare-capacity slice stay correct and unchanged.
func TestSlicesConcurrent(t *testing.T) {
	lib.Record.Reset()
	full := []lib.Option{
		lib.Named("a"), lib.Named("b"),
		lib.Named("s0"), lib.Named("s1"), lib.Named("s2"),
	}
	xs := full[:2]
	Concurrent(xs, 8)
	calls := lib.Record.Names()
	if len(calls) != 8 {
		t.Fatalf("recorded %d calls, want 8", len(calls))
	}
	for i, got := range calls {
		if len(got) != 3 || got[0] != "a" || got[1] != "b" || got[2] != "traced" {
			t.Fatalf("call %d recorded %v, want [a b traced]", i, got)
		}
	}
	if len(xs) != 2 || cap(xs) != 5 {
		t.Fatalf("caller header changed: len=%d cap=%d", len(xs), cap(xs))
	}
	for i, want := range []string{"a", "b", "s0", "s1", "s2"} {
		if full[i].Name != want {
			t.Fatalf("backing element %d = %q, want %q", i, full[i].Name, want)
		}
	}
}

// TestSlicesNonNilWithValues is S09: nil and empty inputs crossed with one and
// three templates always produce a non-nil result with the exact injected
// count.
func TestSlicesNonNilWithValues(t *testing.T) {
	lib.Record.Reset()
	NilOne()
	if hdr := lib.Record.LastHeader(); hdr.Data == nil || hdr.Len != 1 {
		t.Fatalf("nil input with one template: header %+v", hdr)
	}

	lib.Record.Reset()
	EmptyOne(make([]lib.Option, 0))
	if hdr := lib.Record.LastHeader(); hdr.Data == nil || hdr.Len != 1 {
		t.Fatalf("empty input with one template: header %+v", hdr)
	}

	lib.Record.Reset()
	NilMany()
	if hdr := lib.Record.LastHeader(); hdr.Data == nil || hdr.Len != 3 {
		t.Fatalf("nil input with three templates: header %+v", hdr)
	}
	check.LastNames(t, "one", "two", "three")

	lib.Record.Reset()
	EmptyMany(make([]lib.Option, 0))
	if hdr := lib.Record.LastHeader(); hdr.Data == nil || hdr.Len != 3 {
		t.Fatalf("empty input with three templates: header %+v", hdr)
	}
	check.LastNames(t, "one", "two", "three")
}

// TestSlicesLiteral is S10: untyped nil and literal spreads compile (they
// guard the explicit type argument) and reach the callee with the exact
// length.
func TestSlicesLiteral(t *testing.T) {
	lib.Record.Reset()
	LiteralNil()
	if hdr := lib.Record.LastHeader(); hdr.Data == nil || hdr.Len != 1 {
		t.Fatalf("nil literal: header %+v", hdr)
	}
	check.LastNames(t, "traced")

	lib.Record.Reset()
	LiteralEmpty()
	if hdr := lib.Record.LastHeader(); hdr.Data == nil || hdr.Len != 1 {
		t.Fatalf("empty literal: header %+v", hdr)
	}

	lib.Record.Reset()
	LiteralPopulated(lib.Named("a"), lib.Named("b"))
	check.LastNames(t, "a", "b", "traced")
}

// TestSlicesNamed is T01: a defined slice type and a slice alias both spread
// into the same woven sink with the same elements.
func TestSlicesNamed(t *testing.T) {
	lib.Record.Reset()
	SpreadDefined(lib.Options{lib.Named("a")})
	check.LastNames(t, "a", "traced")

	lib.Record.Reset()
	SpreadAlias(lib.OptAlias{lib.Named("b")})
	check.LastNames(t, "b", "traced")
}

// TestSlicesInterface is T03: dynamic types and typed-nil values survive the
// copy without conversion.
func TestSlicesInterface(t *testing.T) {
	lib.Record.Reset()
	box := &lib.Box{Name: "box"}
	typedNil := (*lib.Box)(nil)
	InterfaceMixed([]any{box, typedNil, 1, "s"})
	anys := lib.Record.Anys()
	if len(anys) != 1 || len(anys[0]) != 5 {
		t.Fatalf("recorded %v", anys)
	}
	got := anys[0]
	if got[0] != any(box) {
		t.Fatal("pointer identity lost through the copy")
	}
	if b, ok := got[1].(*lib.Box); !ok || b != nil {
		t.Fatalf("typed nil not preserved: %v", got[1])
	}
	if got[2] != 1 || got[3] != "s" || got[4] != "anyval" {
		t.Fatalf("recorded %v", got)
	}
}

// TestSlicesPointers is T07: pointer elements, including nil, keep their
// identity.
func TestSlicesPointers(t *testing.T) {
	lib.Record.Reset()
	box := &lib.Box{Name: "box"}
	Pointers([]*lib.Box{box, nil})
	ptrs := lib.Record.Ptrs()
	if len(ptrs) != 1 || len(ptrs[0]) != 3 {
		t.Fatalf("recorded %v", ptrs)
	}
	if ptrs[0][0] != box || ptrs[0][1] != nil {
		t.Fatalf("pointer identity changed: %v", ptrs[0])
	}
}
