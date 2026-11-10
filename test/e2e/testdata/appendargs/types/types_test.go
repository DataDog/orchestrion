// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package types

import (
	"testing"

	"example.com/appendargs/lib"
)

// recordedNames extracts the option names from the recorded dynamic values.
func recordedNames(t *testing.T) []string {
	t.Helper()
	anys := lib.Record.Anys()
	if len(anys) != 1 {
		t.Fatalf("recorded %d calls, want 1", len(anys))
	}
	got := make([]string, 0, len(anys[0]))
	for _, v := range anys[0] {
		opt, ok := v.(lib.Option)
		if !ok {
			t.Fatalf("value %#v is not an Option", v)
		}
		got = append(got, opt.Name)
	}
	return got
}

func wantNames(t *testing.T, got []string, want ...string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("recorded %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("recorded %v, want %v", got, want)
		}
	}
}

// TestTypesGeneric is T02: the explicit []T instantiation preserves the actual
// type parameter, and the advice applies to the inferred generic call.
func TestTypesGeneric(t *testing.T) {
	lib.Record.Reset()
	Generic([]lib.Option{lib.Named("x")}, lib.Named("extra"))
	wantNames(t, recordedNames(t), "x", "extra")

	lib.Record.Reset()
	Generic([]string{"s"}, "extra")
	got := lib.Record.Anys()[0]
	if len(got) != 2 || got[0] != "s" || got[1] != "extra" {
		t.Fatalf("recorded %v", got)
	}
}

// TestTypesTypeParamSlice is T08: a type-parameter slice converts to []E for
// the helpers, and the actual element type is preserved.
func TestTypesTypeParamSlice(t *testing.T) {
	lib.Record.Reset()
	TypeParamSlice([]lib.Option{lib.Named("x")}, lib.Named("extra"))
	wantNames(t, recordedNames(t), "x", "extra")
}
