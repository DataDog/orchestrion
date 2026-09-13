// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package taint

import (
	"bytes"
	"io"
)

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

// BufferWriteByte writes a scalar byte and records its taint range.
func BufferWriteByte[B ~byte](buffer *bytes.Buffer, value *B) error {
	ranges := RangesBytes(buffer.Bytes())
	offset := buffer.Len()
	err := buffer.WriteByte(byte(*value))
	if active.byteScalarTainted(scalarAddress(value)) {
		ranges = append(ranges, Range{Start: offset, End: offset + 1})
	}
	registerBytes(buffer.Bytes(), ranges)
	return err
}

// BufferWriteRuneFromByte writes a byte scalar as a rune and records its range.
func BufferWriteRuneFromByte[B ~byte](buffer *bytes.Buffer, value *B) (int, error) {
	ranges := RangesBytes(buffer.Bytes())
	offset := buffer.Len()
	written, err := buffer.WriteRune(rune(*value))
	if active.byteScalarTainted(scalarAddress(value)) {
		ranges = append(ranges, Range{Start: offset, End: offset + written})
	}
	registerBytes(buffer.Bytes(), ranges)
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

// BufferGrow grows the buffer while preserving metadata across a relocation.
func BufferGrow(buffer *bytes.Buffer, size int) {
	ranges := RangesBytes(buffer.Bytes())
	buffer.Grow(size)
	registerBytes(buffer.Bytes(), ranges)
}

// BufferNext returns and removes bytes while shifting remaining metadata.
func BufferNext(buffer *bytes.Buffer, count int) []byte {
	return buffer.Next(count)
}

// BufferRead reads into destination and records ranges on written bytes.
func BufferRead(buffer *bytes.Buffer, destination []byte) (int, error) {
	ranges := RangesBytes(buffer.Bytes())
	read, err := buffer.Read(destination)
	registerBytes(destination[:read], normalizeRanges(ranges, read))
	return read, err
}

// BufferReadBytes reads through delimiter and records ranges on the result.
func BufferReadBytes(buffer *bytes.Buffer, delimiter byte) ([]byte, error) {
	ranges := RangesBytes(buffer.Bytes())
	result, err := buffer.ReadBytes(delimiter)
	registerBytes(result, normalizeRanges(ranges, len(result)))
	return result, err
}

// BufferReadString reads through delimiter and records ranges on the result.
func BufferReadString(buffer *bytes.Buffer, delimiter byte) (string, error) {
	ranges := RangesBytes(buffer.Bytes())
	result, err := buffer.ReadString(delimiter)
	registerString(result, normalizeRanges(ranges, len(result)))
	return result, err
}

// BufferWriteTo transfers ranges into a concrete buffer destination.
func BufferWriteTo(source *bytes.Buffer, destination io.Writer) (int64, error) {
	sourceRanges := RangesBytes(source.Bytes())
	buffer, ok := destination.(*bytes.Buffer)
	if !ok {
		return source.WriteTo(destination)
	}

	destinationRanges := RangesBytes(buffer.Bytes())
	destinationLength := buffer.Len()
	written, err := source.WriteTo(destination)
	ranges := append(destinationRanges, shiftedRanges(normalizeRanges(sourceRanges, int(written)), destinationLength)...)
	registerBytes(buffer.Bytes(), ranges)
	return written, err
}

// BufferReadFrom appends reader data and records ranges for buffer sources.
func BufferReadFrom(buffer *bytes.Buffer, reader io.Reader) (int64, error) {
	destinationRanges := RangesBytes(buffer.Bytes())
	before := buffer.Len()
	var sourceRanges []Range
	if source, ok := reader.(*bytes.Buffer); ok {
		sourceRanges = RangesBytes(source.Bytes())
	}
	read, err := buffer.ReadFrom(reader)
	postRanges := RangesBytes(buffer.Bytes())
	resultRanges := append(destinationRanges, postRanges...)
	resultRanges = append(resultRanges, shiftedRanges(normalizeRanges(sourceRanges, int(read)), before)...)
	registerBytes(buffer.Bytes(), resultRanges)
	return read, err
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
