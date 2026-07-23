// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package taint

import "testing"

func Test_ConversionRoundTripPreservesRanges_when_StringPassesThroughBytes(t *testing.T) {
	// Given
	value := ConcatStrings("safe-", SourceString("bad"))

	// When
	bytes := StringToBytes(value)
	roundTrip := BytesToString(bytes)

	// Then
	if roundTrip != value {
		t.Fatalf("round trip = %q, want %q", roundTrip, value)
	}
	requireRanges(t, RangesBytes(bytes), Range{Start: 5, End: 8})
	requireRanges(t, RangesString(roundTrip), Range{Start: 5, End: 8})
}

func Test_SliceBytesProjectsRanges_when_SliceCutsCleanPrefix(t *testing.T) {
	// Given
	bytes := StringToBytes(ConcatStrings("ab", SourceString("cde")))

	// When
	sliced := SliceBytes(bytes, 1, 5)

	// Then
	if string(sliced) != "bcde" {
		t.Fatalf("SliceBytes() = %q, want %q", sliced, "bcde")
	}
	requireRanges(t, RangesBytes(sliced), Range{Start: 1, End: 4})
}

func Test_AppendBytesShiftsSourceRanges_when_DestinationHasSpareCapacity(t *testing.T) {
	// Given
	destination := make([]byte, 5, 16)
	copy(destination, "safe-")
	source := StringToBytes(SourceString("bad"))

	// When
	result := AppendBytes(destination, source)

	// Then
	if string(result) != "safe-bad" {
		t.Fatalf("AppendBytes() = %q, want %q", result, "safe-bad")
	}
	requireRanges(t, RangesBytes(result), Range{Start: 5, End: 8})
}

func Test_AppendBytesPreservesRanges_when_GrowthRelocatesStorage(t *testing.T) {
	// Given
	destination := StringToBytes(SourceString("old"))
	source := []byte("-clean")

	// When
	result := AppendBytes(destination[:len(destination):len(destination)], source)

	// Then
	if string(result) != "old-clean" {
		t.Fatalf("AppendBytes() = %q, want %q", result, "old-clean")
	}
	requireRanges(t, RangesBytes(result), Range{Start: 0, End: 3})
}

func Test_CopyBytesCopiesAndClearsRanges_when_DestinationIsOverwritten(t *testing.T) {
	// Given
	destination := []byte("0123456789")
	tainted := StringToBytes(SourceString("bad"))

	// When
	copied := CopyBytes(destination[3:6], tainted)

	// Then
	if copied != 3 || string(destination) != "012bad6789" {
		t.Fatalf("CopyBytes() = %d, destination = %q", copied, destination)
	}
	requireRanges(t, RangesBytes(destination), Range{Start: 3, End: 6})

	// When
	CopyBytes(destination[4:5], []byte("X"))

	// Then
	requireRanges(t, RangesBytes(destination),
		Range{Start: 3, End: 4},
		Range{Start: 5, End: 6},
	)
}

func Test_CloneBytesCopiesRanges_when_ResultHasIndependentStorage(t *testing.T) {
	// Given
	value := StringToBytes(ConcatStrings("clean", SourceString("bad")))

	// When
	cloned := CloneBytes(value)

	// Then
	requireRanges(t, RangesBytes(cloned), Range{Start: 5, End: 8})
}
