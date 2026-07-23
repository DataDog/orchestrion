// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package taint

import "unicode/utf8"

// StringToRunesAs converts a string into runes and projects byte taint to rune indexes.
func StringToRunesAs[T ~[]E, E ~rune, S ~string](value S) T {
	runes := []rune(value)
	result := make(T, len(runes))
	for index := range runes {
		result[index] = E(runes[index])
	}
	inputRanges := RangesString(string(value))
	ranges := make([]Range, 0, len(inputRanges))
	byteOffset := 0
	for runeIndex := range result {
		_, size := utf8.DecodeRuneInString(string(value)[byteOffset:])
		if rangesOverlap(inputRanges, byteOffset, byteOffset+size) {
			ranges = append(ranges, Range{Start: runeIndex, End: runeIndex + 1})
		}
		byteOffset += size
	}
	registerRunes(result, ranges)
	return result
}

// RunesToStringAs converts runes into a string and conservatively propagates taint.
func RunesToStringAs[T ~string, R ~[]E, E ~rune](value R) T {
	runes := make([]rune, len(value))
	for index := range value {
		runes[index] = rune(value[index])
	}
	result := T(isolatedString(string(runes)))
	if len(RangesRunes(value)) > 0 {
		registerString(string(result), []Range{{Start: 0, End: len(result)}})
	}
	return result
}

func rangesOverlap(ranges []Range, start, end int) bool {
	for _, current := range ranges {
		if max(current.Start, start) < min(current.End, end) {
			return true
		}
	}
	return false
}
