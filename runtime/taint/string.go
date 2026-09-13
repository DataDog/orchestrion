// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package taint

import (
	"strings"
	"unicode/utf8"
)

// SourceString marks every byte in a fresh occurrence of value as tainted.
func SourceString(value string) string {
	result := isolatedString(value)
	registerFreshString(result, newSourceID())
	return result
}

// ConcatStrings concatenates two strings and propagates their byte ranges.
func ConcatStrings[T ~string](left, right T) T {
	leftRanges := RangesString(string(left))
	rightRanges := shiftedRanges(RangesString(string(right)), len(left))
	result := left + right
	registerString(string(result), append(leftRanges, rightRanges...))
	return result
}

// SliceString slices value and projects ranges onto the result.
func SliceString[T ~string](value T, low, high int) T {
	result := value[low:high]
	registerString(string(result), RangesString(string(result)))
	return result
}

// StringByteAt converts one indexed string byte and propagates its source range.
func StringByteAt[T ~string](value T, index int) string {
	return StringByteAtAs[string](value, index)
}

// StringByteAtAs converts one indexed byte into a string type with propagation.
func StringByteAtAs[T ~string, S ~string](value S, index int) T {
	result := T(isolatedString(string(value[index])))
	if rangesContain(RangesString(string(value)), index) {
		registerString(string(result), []Range{{Start: 0, End: len(result)}})
	}
	return result
}

// CloneString returns an independent string occurrence with copied ranges.
func CloneString(value string) string {
	result := isolatedString(value)
	registerString(result, RangesString(value))
	return result
}

// ReplaceString applies strings.Replace and propagates consumed byte ranges.
func ReplaceString(value, old, replacement string, count int) string {
	rawResult := strings.Replace(value, old, replacement, count)
	result := isolatedString(rawResult)
	valueRanges := RangesString(value)
	replacementRanges := RangesString(replacement)
	if old == "" {
		registerString(result, replaceEmptyRanges(value, replacement, count, valueRanges, replacementRanges))
		return result
	}
	registerString(result, replaceRanges(value, old, replacement, count, valueRanges, replacementRanges))
	return result
}

func replaceRanges(value, old, replacement string, count int, valueRanges, replacementRanges []Range) []Range {
	remaining := count
	if remaining < 0 {
		remaining = len(value) + 1
	}
	result := make([]Range, 0, len(valueRanges)+len(replacementRanges))
	sourcePosition := 0
	destinationPosition := 0
	for remaining > 0 {
		relative := strings.Index(value[sourcePosition:], old)
		if relative < 0 {
			break
		}
		matchStart := sourcePosition + relative
		result = append(result, projectRanges(valueRanges, sourcePosition, matchStart, destinationPosition)...)
		destinationPosition += matchStart - sourcePosition
		result = append(result, shiftedRanges(replacementRanges, destinationPosition)...)
		destinationPosition += len(replacement)
		sourcePosition = matchStart + len(old)
		remaining--
	}
	return append(result, projectRanges(valueRanges, sourcePosition, len(value), destinationPosition)...)
}

func replaceEmptyRanges(value, replacement string, count int, valueRanges, replacementRanges []Range) []Range {
	remaining := count
	if remaining < 0 {
		remaining = len([]rune(value)) + 1
	}
	result := make([]Range, 0, len(valueRanges)+len(replacementRanges))
	sourcePosition := 0
	destinationPosition := 0
	for remaining > 0 {
		result = append(result, shiftedRanges(replacementRanges, destinationPosition)...)
		destinationPosition += len(replacement)
		remaining--
		if sourcePosition == len(value) {
			break
		}
		_, runeSize := utf8.DecodeRuneInString(value[sourcePosition:])
		result = append(result, projectRanges(valueRanges, sourcePosition, sourcePosition+runeSize, destinationPosition)...)
		sourcePosition += runeSize
		destinationPosition += runeSize
	}
	return append(result, projectRanges(valueRanges, sourcePosition, len(value), destinationPosition)...)
}

func projectRanges(ranges []Range, sourceStart, sourceEnd, destinationStart int) []Range {
	result := make([]Range, 0, len(ranges))
	for _, current := range ranges {
		start := max(current.Start, sourceStart)
		end := min(current.End, sourceEnd)
		if start < end {
			result = append(result, Range{
				Start:    destinationStart + start - sourceStart,
				End:      destinationStart + end - sourceStart,
				SourceID: current.SourceID,
			})
		}
	}
	return result
}

func rangesContain(ranges []Range, index int) bool {
	for _, current := range ranges {
		if current.Start <= index && index < current.End {
			return true
		}
	}
	return false
}
