// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package taint

import (
	"strings"
	"testing"
)

type namedString string

func Test_StringAdaptersReturnBuiltinString_when_InputIsNamed(t *testing.T) {
	input := namedString("mixed")
	results := []any{
		CloneString(string(input)),
		ReplaceString(string(input), "x", "y", 1),
		ReplaceAllString(string(input), "x", "y"),
		RepeatString(string(input), 2),
		UpperString(string(input)),
		LowerString(string(input)),
		MapString(func(value rune) rune { return value }, string(input)),
	}
	for _, result := range results {
		if _, ok := result.(string); !ok {
			t.Fatalf("adapter returned %T, want string", result)
		}
	}
}

func Test_EqualCleanStringStaysClean_when_ContentMatchesTaintedString(t *testing.T) {
	// Given
	tainted := SourceString(strings.Clone("same-value"))
	clean := strings.Clone(string(append([]byte(nil), "same-value"...)))

	// When
	taintedRanges := RangesString(tainted)
	cleanRanges := RangesString(clean)

	// Then
	requireRanges(t, taintedRanges, Range{Start: 0, End: len(tainted)})
	requireRanges(t, cleanRanges)
}

func Test_ConcatStringsShiftsRanges_when_OnlyMiddleOperandIsTainted(t *testing.T) {
	// Given
	middle := SourceString("bad")

	// When
	value := ConcatStrings(ConcatStrings("safe-", middle), "-safe")

	// Then
	if value != "safe-bad-safe" {
		t.Fatalf("ConcatStrings() = %q, want %q", value, "safe-bad-safe")
	}
	requireRanges(t, RangesString(value), Range{Start: 5, End: 8})
}

func Test_SliceStringProjectsRanges_when_SliceCutsBothCleanSides(t *testing.T) {
	// Given
	value := ConcatStrings(ConcatStrings("ab", SourceString("cde")), "fg")

	// When
	sliced := SliceString(value, 1, 6)

	// Then
	if sliced != "bcdef" {
		t.Fatalf("SliceString() = %q, want %q", sliced, "bcdef")
	}
	requireRanges(t, RangesString(sliced), Range{Start: 1, End: 4})
}

func Test_CloneStringCopiesRanges_when_ResultHasIndependentStorage(t *testing.T) {
	// Given
	value := ConcatStrings("clean", SourceString("tainted"))

	// When
	cloned := CloneString(value)

	// Then
	if cloned != value {
		t.Fatalf("CloneString() = %q, want %q", cloned, value)
	}
	requireRanges(t, RangesString(cloned), Range{Start: 5, End: 12})
}

func Test_ReplaceStringDropsRemovedRanges_when_TaintedBytesAreReplacedByCleanBytes(t *testing.T) {
	// Given
	value := ConcatStrings(ConcatStrings("before-", SourceString("secret")), "-after")

	// When
	replaced := ReplaceString(value, "secret", "public", 1)

	// Then
	if replaced != "before-public-after" {
		t.Fatalf("ReplaceString() = %q, want %q", replaced, "before-public-after")
	}
	requireRanges(t, RangesString(replaced))
}

func Test_ReplaceStringCopiesReplacementRanges_when_ReplacementIsTainted(t *testing.T) {
	// Given
	replacement := SourceString("secret")

	// When
	replaced := ReplaceString("before-value-after", "value", replacement, 1)

	// Then
	requireRanges(t, RangesString(replaced), Range{Start: 7, End: 13})
}

func Test_ReplaceStringHandlesInvalidUTF8_when_OldValueIsEmpty(t *testing.T) {
	// Given
	value := SourceString("\xffa")

	// When
	replaced := ReplaceString(value, "", "-", -1)

	// Then
	if replaced != "-\xff-a-" {
		t.Fatalf("ReplaceString() = %q, want %q", replaced, "-\xff-a-")
	}
	if len(RangesString(replaced)) == 0 {
		t.Fatal("ReplaceString() lost taint")
	}
}

func Test_ReplaceStringDoesNotClearInput_when_EqualReplacementAliasesInput(t *testing.T) {
	// Given
	value := SourceString("secret")

	// When
	_ = ReplaceString(value, "secret", "secret", 1)

	// Then
	requireRanges(t, RangesString(value), Range{Start: 0, End: 6})
}

func Test_BuilderStringAggregatesRanges_when_WritesMixCleanAndTaintedStrings(t *testing.T) {
	// Given
	var builder strings.Builder

	// When
	_, _ = BuilderWriteString(&builder, "safe-")
	_, _ = BuilderWriteString(&builder, SourceString("bad"))
	_, _ = BuilderWriteString(&builder, "-safe")
	value := BuilderString(&builder)

	// Then
	if value != "safe-bad-safe" {
		t.Fatalf("BuilderString() = %q, want %q", value, "safe-bad-safe")
	}
	requireRanges(t, RangesString(value), Range{Start: 5, End: 8})
}

func requireRanges(t *testing.T, actual []Range, expected ...Range) {
	t.Helper()
	if len(actual) != len(expected) {
		t.Fatalf("ranges = %#v, want %#v", actual, expected)
	}
	for index := range expected {
		if actual[index] != expected[index] {
			t.Fatalf("ranges[%d] = %#v, want %#v", index, actual[index], expected[index])
		}
	}
}
