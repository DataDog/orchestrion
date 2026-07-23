// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package taint

import "bytes"

// NewBuffer tracks a new buffer initialized from value.
func NewBuffer(value []byte) *bytes.Buffer {
	buffer := bytes.NewBuffer(value)
	ranges := RangesBytes(value)
	registerBytes(buffer.Bytes(), ranges)
	return buffer
}

// NewBufferString tracks a new buffer initialized from value.
func NewBufferString(value string) *bytes.Buffer {
	buffer := bytes.NewBufferString(value)
	ranges := RangesString(value)
	registerBytes(buffer.Bytes(), ranges)
	return buffer
}

// BufferWriteString writes value and records its ranges at the written offset.
func BufferWriteString(buffer *bytes.Buffer, value string) (int, error) {
	ranges := RangesString(value)
	before := buffer.Len()
	currentRanges := RangesBytes(buffer.Bytes())
	written, err := buffer.WriteString(value)
	resultRanges := append(currentRanges, shiftedRanges(normalizeRanges(ranges, written), before)...)
	registerBytes(buffer.Bytes(), resultRanges)
	return written, err
}

// BufferWrite writes value and records its ranges at the written offset.
func BufferWrite(buffer *bytes.Buffer, value []byte) (int, error) {
	ranges := RangesBytes(value)
	before := buffer.Len()
	currentRanges := RangesBytes(buffer.Bytes())
	written, err := buffer.Write(value)
	resultRanges := append(currentRanges, shiftedRanges(normalizeRanges(ranges, written), before)...)
	registerBytes(buffer.Bytes(), resultRanges)
	return written, err
}

// BufferString returns the buffer contents and attaches a metadata snapshot.
func BufferString(buffer *bytes.Buffer) string {
	result := buffer.String()
	registerString(result, RangesBytes(buffer.Bytes()))
	return result
}

// BufferBytes returns the current bytes and attaches a metadata snapshot.
func BufferBytes(buffer *bytes.Buffer) []byte {
	return buffer.Bytes()
}

// BufferNext returns and removes bytes while shifting remaining metadata.
func BufferNext(buffer *bytes.Buffer, count int) []byte {
	return buffer.Next(count)
}

// BufferTruncate truncates the buffer and its metadata.
func BufferTruncate(buffer *bytes.Buffer, length int) {
	before := buffer.Bytes()
	ranges := normalizeRanges(RangesBytes(before), length)
	buffer.Truncate(length)
	registerBytes(before, ranges)
}

// BufferReset resets the buffer and clears its metadata.
func BufferReset(buffer *bytes.Buffer) {
	before := buffer.Bytes()
	buffer.Reset()
	registerBytes(before, nil)
}
