// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package taint

import "testing"

func Test_StringDestinationScannerCopiesByteRanges_when_SourceIsTainted(t *testing.T) {
	// Given
	var target string
	scanner := stringDestinationScanner{target: &target}

	// When
	err := scanner.Scan(StringToBytes(SourceString("secret")))

	// Then
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	if target != "secret" {
		t.Fatalf("Scan() target = %q, want %q", target, "secret")
	}
	requireRanges(t, RangesString(target), Range{Start: 0, End: 6})
}
