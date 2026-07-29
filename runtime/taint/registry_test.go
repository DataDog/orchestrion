// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package taint

import (
	"strconv"
	"testing"
)

func Test_RegistryStringKeepsExactRangesPast65536(t *testing.T) {
	// Given
	local := newRegistry()
	seedStringOwners(t, &local)
	if len(local.stringOwners) != maxTrackedOccurrences {
		t.Fatalf("string owner count = %d, want %d", len(local.stringOwners), maxTrackedOccurrences)
	}

	// When
	overflow := isolatedString("overflow")
	local.registerString(overflow, []Range{{Start: 0, End: len(overflow)}})

	// Then
	if len(local.stringOwners) != maxTrackedOccurrences+1 {
		t.Fatalf("string owner count after overflow = %d, want %d", len(local.stringOwners), maxTrackedOccurrences+1)
	}
	overflowStart, ok := stringAddress(overflow)
	if !ok {
		t.Fatal("overflow has no storage start")
	}
	requireRanges(t, relativeRanges(local.stringRanges, overflowStart, len(overflow)), Range{Start: 0, End: len(overflow)})

	clean := isolatedString(overflow)
	if clean != overflow {
		t.Fatalf("clean value = %q, want %q", clean, overflow)
	}
	cleanStart, ok := stringAddress(clean)
	if !ok {
		t.Fatal("clean value has no storage start")
	}
	if cleanStart == overflowStart {
		t.Fatal("clean value aliases the overflow storage")
	}
	if ranges := relativeRanges(local.stringRanges, cleanStart, len(clean)); len(ranges) != 0 {
		t.Fatalf("clean value ranges = %#v, want none", ranges)
	}
}

func seedStringOwners(t *testing.T, registry *registry) {
	t.Helper()
	for index := 0; index < maxTrackedOccurrences; index++ {
		value := isolatedString(strconv.Itoa(index))
		start, ok := stringAddress(value)
		if !ok {
			t.Fatalf("seed value %q has no storage start", value)
		}
		registry.stringOwners[start] = value
	}
}
