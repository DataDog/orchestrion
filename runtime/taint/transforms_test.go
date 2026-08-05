// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package taint

import "testing"

func Test_CallStringTransformConservativelyTaintsFreshResult_when_InputIsTainted(t *testing.T) {
	// Given
	calls := 0
	freshCopy := func(input string) string {
		calls++
		copied := make([]byte, len(input))
		copy(copied, input)
		return string(copied)
	}

	// When
	tainted := CallStringTransform(freshCopy, SourceString("secret"))
	clean := CallStringTransform(freshCopy, "clean")

	// Then
	if tainted != "secret" {
		t.Fatalf("CallStringTransform() = %q, want %q", tainted, "secret")
	}
	if clean != "clean" {
		t.Fatalf("CallStringTransform() = %q, want %q", clean, "clean")
	}
	if calls != 2 {
		t.Fatalf("transform calls = %d, want 2", calls)
	}
	requireRanges(t, RangesString(tainted), Range{Start: 0, End: len(tainted)})
	requireRanges(t, RangesString(clean))
}
