// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package taint

import (
	"bytes"
	"testing"
)

func Test_BufioReadStringProjectsSourceRanges_when_ReaderHasCleanPrefixAndDelimiter(t *testing.T) {
	// Given
	value := ConcatStrings(ConcatStrings("safe-", SourceString("secret")), "\n")
	reader := NewBufioReader(NewStringReader(value))

	// When
	result, err := BufioReadString(reader, '\n')

	// Then
	if err != nil {
		t.Fatalf("BufioReadString() error = %v", err)
	}
	if result != "safe-secret\n" {
		t.Fatalf("BufioReadString() = %q, want %q", result, "safe-secret\n")
	}
	requireRanges(t, RangesString(result), Range{Start: 5, End: 11})
}

func Test_IOCopyProjectsSourceRanges_when_ReaderHasCleanPrefix(t *testing.T) {
	// Given
	source := ConcatStrings("safe-", SourceString("secret"))
	var destination bytes.Buffer

	// When
	written, err := IOCopy(&destination, NewStringReader(source))

	// Then
	if err != nil {
		t.Fatalf("IOCopy() error = %v", err)
	}
	if written != 11 || destination.String() != "safe-secret" {
		t.Fatalf("IOCopy() = (%d, %q), want (%d, %q)", written, destination.String(), 11, "safe-secret")
	}
	requireRanges(t, RangesBytes(destination.Bytes()), Range{Start: 5, End: 11})
}
