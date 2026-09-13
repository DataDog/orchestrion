// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package taint

import (
	"encoding/base64"
	"regexp"
	"strconv"
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

func Test_SourceStringAssignsDistinctSourceIDs_when_ValuesHaveSameBytes(t *testing.T) {
	// Given
	first := SourceString("same-value")
	second := SourceString("same-value")

	// When
	firstRanges := RangesString(first)
	secondRanges := RangesString(second)

	// Then
	requireRanges(t, firstRanges, Range{Start: 0, End: len(first)})
	requireRanges(t, secondRanges, Range{Start: 0, End: len(second)})
	if firstRanges[0].SourceID == 0 || secondRanges[0].SourceID == 0 {
		t.Fatalf("source IDs = %d, %d, want nonzero IDs", firstRanges[0].SourceID, secondRanges[0].SourceID)
	}
	if firstRanges[0].SourceID == secondRanges[0].SourceID {
		t.Fatalf("source IDs = %d, %d, want distinct IDs", firstRanges[0].SourceID, secondRanges[0].SourceID)
	}
}

func Test_UpperStringRetainsAllSourceIDs_when_InputHasMultipleRoots(t *testing.T) {
	// Given
	left := SourceString("alpha")
	right := SourceString("beta")
	input := ConcatStrings(left, right)
	inputRanges := RangesString(input)

	// When
	result := UpperString(input)
	resultRanges := RangesString(result)

	// Then
	if result != "ALPHABETA" {
		t.Fatalf("UpperString() = %q, want %q", result, "ALPHABETA")
	}
	requireRanges(t, inputRanges, Range{Start: 0, End: 5}, Range{Start: 5, End: 9})
	requireRanges(t, resultRanges, Range{Start: 0, End: len(result)}, Range{Start: 0, End: len(result)})
	inputIDs := map[uint64]struct{}{inputRanges[0].SourceID: {}, inputRanges[1].SourceID: {}}
	resultIDs := map[uint64]struct{}{resultRanges[0].SourceID: {}, resultRanges[1].SourceID: {}}
	if len(inputIDs) != 2 || len(resultIDs) != 2 {
		t.Fatalf("input IDs = %#v, result IDs = %#v, want two IDs each", inputIDs, resultIDs)
	}
	for sourceID := range inputIDs {
		if _, found := resultIDs[sourceID]; !found {
			t.Fatalf("result IDs = %#v, missing input source ID %d", resultIDs, sourceID)
		}
	}
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

func Test_QuoteStringConservativelyTaintsFreshResult_when_InputIsTainted(t *testing.T) {
	// Given
	value := SourceString("secret")

	// When
	quoted := QuoteString(value)

	// Then
	if quoted != strconv.Quote("secret") {
		t.Fatalf("QuoteString() = %q, want %q", quoted, strconv.Quote("secret"))
	}
	requireRanges(t, RangesString(quoted), Range{Start: 0, End: len(quoted)})
}

func Test_QueryEscapeStringConservativelyTaintsFreshResult_when_InputIsTainted(t *testing.T) {
	// Given
	value := SourceString("/secret")

	// When
	escaped := QueryEscapeString(value)

	// Then
	if escaped != "%2Fsecret" {
		t.Fatalf("QueryEscapeString() = %q, want %q", escaped, "%2Fsecret")
	}
	requireRanges(t, RangesString(escaped), Range{Start: 0, End: 9})
}

func Test_RegexpReplaceAllStringConservativelyTaintsFreshResult_when_SourceIsTainted(t *testing.T) {
	// Given
	expression := regexp.MustCompile("value")
	source := SourceString("secret-value")

	// When
	replaced := RegexpReplaceAllString(expression, source, "public")

	// Then
	if replaced != "secret-public" {
		t.Fatalf("RegexpReplaceAllString() = %q, want %q", replaced, "secret-public")
	}
	requireRanges(t, RangesString(replaced), Range{Start: 0, End: 13})
}

func Test_Base64EncodeToStringConservativelyTaintsFreshResult_when_SourceBytesAreTainted(t *testing.T) {
	// Given
	source := StringToBytes(SourceString("secret"))

	// When
	encoded := Base64EncodeToString(base64.StdEncoding, source)

	// Then
	if encoded != "c2VjcmV0" {
		t.Fatalf("Base64EncodeToString() = %q, want %q", encoded, "c2VjcmV0")
	}
	requireRanges(t, RangesString(encoded), Range{Start: 0, End: 8})
}

func Test_CleanPathConservativelyTaintsFreshResult_when_InputIsTainted(t *testing.T) {
	// Given
	value := SourceString("a/../secret")

	// When
	cleaned := CleanPath(value)

	// Then
	if cleaned != "secret" {
		t.Fatalf("CleanPath() = %q, want %q", cleaned, "secret")
	}
	requireRanges(t, RangesString(cleaned), Range{Start: 0, End: 6})
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

func Test_BuilderWriteAppendsByteRanges_when_SourceHasCleanPrefix(t *testing.T) {
	// Given
	var builder strings.Builder

	// When
	_, _ = BuilderWriteString(&builder, "builder-")
	_, _ = BuilderWrite(&builder, StringToBytes(SourceString("secret")))
	value := BuilderString(&builder)

	// Then
	if value != "builder-secret" {
		t.Fatalf("BuilderString() = %q, want %q", value, "builder-secret")
	}
	requireRanges(t, RangesString(value), Range{Start: 8, End: 14})
}

func requireRanges(t *testing.T, actual []Range, expected ...Range) {
	t.Helper()
	if len(actual) != len(expected) {
		t.Fatalf("ranges = %#v, want %#v", actual, expected)
	}
	for index := range expected {
		if actual[index].Start != expected[index].Start || actual[index].End != expected[index].End {
			t.Fatalf("ranges[%d] = (%d, %d), want (%d, %d)", index, actual[index].Start, actual[index].End, expected[index].Start, expected[index].End)
		}
	}
}
