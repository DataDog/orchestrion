// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package taint

import "strings"

// ReplaceAllString applies strings.ReplaceAll with range propagation.
func ReplaceAllString(value, old, replacement string) string {
	return ReplaceString(value, old, replacement, -1)
}

// RepeatString repeats value and its ranges count times.
func RepeatString(value string, count int) string {
	result := strings.Repeat(value, count)
	inputRanges := RangesString(value)
	ranges := make([]Range, 0, len(inputRanges)*max(count, 0))
	for index := range count {
		ranges = append(ranges, shiftedRanges(inputRanges, index*len(value))...)
	}
	registerString(string(result), ranges)
	return result
}

// UpperString uppercases value and conservatively taints the full result.
func UpperString(value string) string {
	result := strings.ToUpper(value)
	registerTransformedString(result, value)
	return result
}

// LowerString lowercases value and conservatively taints the full result.
func LowerString(value string) string {
	result := strings.ToLower(value)
	registerTransformedString(result, value)
	return result
}

// MapString maps value and conservatively taints the full result.
func MapString(mapping func(rune) rune, value string) string {
	result := strings.Map(mapping, value)
	registerTransformedString(result, value)
	return result
}

func registerTransformedString(result, input string) {
	if len(RangesString(input)) > 0 {
		registerString(result, []Range{{Start: 0, End: len(result)}})
	}
}
