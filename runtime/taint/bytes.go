// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package taint

import "bytes"

// StringToBytes converts a string and copies its tainted byte ranges.
func StringToBytes(value string) []byte {
	return StringToBytesAs[[]byte](value)
}

// StringToBytesAs converts a string into a byte-slice type and copies its ranges.
func StringToBytesAs[T ~[]E, E ~byte, S ~string](value S) T {
	result := make(T, len(value))
	for index := range len(value) {
		result[index] = E(value[index])
	}
	registerBytes(result, RangesString(string(value)))
	return result
}

// BytesToString converts bytes into an independent string and copies their ranges.
func BytesToString(value []byte) string {
	return BytesToStringAs[string](value)
}

// BytesToStringAs converts bytes into a string type and copies their ranges.
func BytesToStringAs[T ~string, B ~[]E, E ~byte](value B) T {
	buffer := make([]byte, len(value))
	for index := range value {
		buffer[index] = byte(value[index])
	}
	result := T(isolatedString(string(buffer)))
	registerString(string(result), RangesBytes(value))
	return result
}

// SliceBytes slices value and projects ranges onto the result.
func SliceBytes[T ~[]E, E ~byte](value T, low, high int) T {
	result := value[low:high]
	registerBytes(result, RangesBytes(result))
	return result
}

// BytesByteAt converts one indexed byte and propagates its source range.
func BytesByteAt[T ~[]E, E ~byte](value T, index int) string {
	return BytesByteAtAs[string](value, index)
}

// BytesByteAtAs converts one indexed byte into a string type with propagation.
func BytesByteAtAs[T ~string, B ~[]E, E ~byte](value B, index int) T {
	result := T(isolatedString(string(rune(value[index]))))
	if rangesContain(RangesBytes(value), index) {
		registerString(string(result), []Range{{Start: 0, End: len(result)}})
	}
	return result
}

// AppendBytes appends source and propagates destination and source ranges.
func AppendBytes[D ~[]E, S ~[]E, E ~byte](destination D, source S) D {
	destinationRanges := RangesBytes(destination)
	sourceRanges := shiftedRanges(RangesBytes(source), len(destination))
	result := append(destination, source...)
	registerBytes(result, append(destinationRanges, sourceRanges...))
	return result
}

// AppendByteValues appends clean scalar bytes while preserving destination ranges.
func AppendByteValues[T ~[]E, E ~byte](destination T, values ...E) T {
	ranges := RangesBytes(destination)
	result := append(destination, values...)
	registerBytes(result, ranges)
	return result
}

// AppendStringBytes appends string bytes and propagates their ranges.
func AppendStringBytes[D ~[]E, E ~byte, S ~string](destination D, source S) D {
	destinationRanges := RangesBytes(destination)
	sourceRanges := shiftedRanges(RangesString(string(source)), len(destination))
	result := destination
	for index := range len(source) {
		result = append(result, E(source[index]))
	}
	registerBytes(result, append(destinationRanges, sourceRanges...))
	return result
}

// CopyBytes copies source into destination and replaces metadata for written bytes.
func CopyBytes[D ~[]E, S ~[]E, E ~byte](destination D, source S) int {
	sourceRanges := RangesBytes(source)
	copied := copy(destination, source)
	registerBytes(destination[:copied], normalizeRanges(sourceRanges, copied))
	return copied
}

// CopyStringToBytes copies string bytes and replaces metadata for written bytes.
func CopyStringToBytes[D ~[]E, E ~byte, S ~string](destination D, source S) int {
	sourceRanges := RangesString(string(source))
	copied := min(len(destination), len(source))
	for index := range copied {
		destination[index] = E(source[index])
	}
	registerBytes(destination[:copied], normalizeRanges(sourceRanges, copied))
	return copied
}

// ClearBytes clears value and removes all ranges on the cleared bytes.
func ClearBytes[T ~[]E, E ~byte](value T) {
	clear(value)
	registerBytes(value, nil)
}

// SetByte writes a clean scalar byte and removes taint from that position.
func SetByte[T ~[]E, E ~byte](value T, index int, replacement E) {
	value[index] = replacement
	registerBytes(value[index:index+1], nil)
}

// SetByteFromBytes copies one indexed byte and its taint state.
func SetByteFromBytes[D ~[]E, S ~[]E, E ~byte](destination D, destinationIndex int, source S, sourceIndex int) {
	destination[destinationIndex] = source[sourceIndex]
	if rangesContain(RangesBytes(source), sourceIndex) {
		registerBytes(destination[destinationIndex:destinationIndex+1], []Range{{Start: 0, End: 1}})
		return
	}
	registerBytes(destination[destinationIndex:destinationIndex+1], nil)
}

// SetByteFromString copies one indexed string byte and its taint state.
func SetByteFromString[D ~[]E, E ~byte, S ~string](destination D, destinationIndex int, source S, sourceIndex int) {
	destination[destinationIndex] = E(source[sourceIndex])
	if rangesContain(RangesString(string(source)), sourceIndex) {
		registerBytes(destination[destinationIndex:destinationIndex+1], []Range{{Start: 0, End: 1}})
		return
	}
	registerBytes(destination[destinationIndex:destinationIndex+1], nil)
}

// CloneBytes returns an independent byte slice with copied ranges.
func CloneBytes(value []byte) []byte {
	result := bytes.Clone(value)
	registerBytes(result, RangesBytes(value))
	return result
}
