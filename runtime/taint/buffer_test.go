// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package taint

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_BufferReadStringProjectsRanges_when_ResultHasCleanPrefix(t *testing.T) {
	// Given
	var buffer bytes.Buffer
	_, err := BufferWriteString(&buffer, "safe-")
	require.NoError(t, err)
	_, err = BufferWriteString(&buffer, SourceString("secret"))
	require.NoError(t, err)
	_, err = BufferWriteString(&buffer, "\nremaining")
	require.NoError(t, err)

	// When
	result, err := BufferReadString(&buffer, '\n')

	// Then
	require.NoError(t, err)
	require.Equal(t, "safe-secret\n", result)
	requireRanges(t, RangesString(result), Range{Start: 5, End: 11})
}

func Test_BufferReadReplacesDestinationRanges_when_DestinationIsOverwritten(t *testing.T) {
	// Given
	source := NewBufferString(ConcatStrings("safe-", SourceString("secret")))
	destination := StringToBytes(ConcatStrings("clean", SourceString("-stale")))

	// When
	read, err := BufferRead(source, destination)

	// Then
	require.NoError(t, err)
	require.Equal(t, len(destination), read)
	require.Equal(t, "safe-secret", string(destination))
	requireRanges(t, RangesBytes(destination), Range{Start: 5, End: 11})
}

func Test_BufferReadBytesProjectsRanges_when_ResultHasCleanPrefix(t *testing.T) {
	// Given
	source := NewBufferString(ConcatStrings(ConcatStrings("safe-", SourceString("secret")), "\nremaining"))

	// When
	result, err := BufferReadBytes(source, '\n')

	// Then
	require.NoError(t, err)
	require.Equal(t, []byte("safe-secret\n"), result)
	requireRanges(t, RangesBytes(result), Range{Start: 5, End: 11})
}

func Test_BufferReadFromPreservesAndTransfersRanges_when_SourceIsBuffer(t *testing.T) {
	// Given
	target := NewBufferString(SourceString("old-"))
	source := NewBufferString(SourceString("secret"))

	// When
	read, err := BufferReadFrom(target, source)

	// Then
	require.NoError(t, err)
	require.EqualValues(t, len("secret"), read)
	require.Equal(t, "old-secret", target.String())
	requireRanges(t, RangesBytes(target.Bytes()), Range{Start: 0, End: 4}, Range{Start: 4, End: 10})
}

func Test_BufferWriteToProjectsRangesAndDrainsSource_when_DestinationIsBuffer(t *testing.T) {
	// Given
	source := NewBufferString(ConcatStrings("safe-", SourceString("secret")))
	var destination bytes.Buffer

	// When
	written, err := BufferWriteTo(source, &destination)

	// Then
	require.NoError(t, err)
	require.EqualValues(t, len("safe-secret"), written)
	require.Equal(t, "safe-secret", destination.String())
	requireRanges(t, RangesBytes(destination.Bytes()), Range{Start: 5, End: 11})
	require.Zero(t, source.Len())
}

func Test_BufferWriteByteRecordsScalarRange_when_ValueIsTainted(t *testing.T) {
	// Given
	var buffer bytes.Buffer
	value := byte('s')
	MarkByteFromString(&value, SourceString("s"), 0)
	t.Cleanup(func() { ReleaseByte(&value) })

	// When
	err := BufferWriteByte(&buffer, &value)

	// Then
	require.NoError(t, err)
	require.Equal(t, "s", buffer.String())
	requireRanges(t, RangesBytes(buffer.Bytes()), Range{Start: 0, End: 1})
}

func Test_BufferWriteRuneFromByteCoversUTF8Encoding_when_ValueIsTainted(t *testing.T) {
	// Given
	var buffer bytes.Buffer
	value := byte(0xff)
	MarkByteFromString(&value, SourceString("\xff"), 0)
	t.Cleanup(func() { ReleaseByte(&value) })

	// When
	written, err := BufferWriteRuneFromByte(&buffer, &value)

	// Then
	require.NoError(t, err)
	require.Equal(t, 2, written)
	require.Equal(t, "\u00ff", buffer.String())
	requireRanges(t, RangesBytes(buffer.Bytes()), Range{Start: 0, End: 2})
}
