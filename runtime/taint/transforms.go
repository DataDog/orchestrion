// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package taint

import (
	"encoding/base64"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"
)

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

// QuoteString quotes value and conservatively taints the full result.
func QuoteString(value string) string {
	result := strconv.Quote(value)
	registerTransformedString(result, value)
	return result
}

// QueryEscapeString escapes value for a URL query and conservatively taints the full result.
func QueryEscapeString(value string) string {
	result := url.QueryEscape(value)
	registerTransformedString(result, value)
	return result
}

// RegexpReplaceAllString applies expression to source and conservatively taints the full result.
func RegexpReplaceAllString(expression *regexp.Regexp, source, replacement string) string {
	result := expression.ReplaceAllString(source, replacement)
	registerTransformedString(result, source)
	return result
}

// Base64EncodeToString encodes source and conservatively taints the full result.
func Base64EncodeToString(encoding *base64.Encoding, source []byte) string {
	result := encoding.EncodeToString(source)
	if ranges := conservativeRanges(len(result), RangesBytes(source)); len(ranges) > 0 {
		registerString(result, ranges)
	}
	return result
}

// CleanPath cleans value and conservatively taints the full result.
func CleanPath(value string) string {
	result := path.Clean(value)
	registerTransformedString(result, value)
	return result
}

// CallStringTransform applies a known fresh-output function and conservatively taints its result.
func CallStringTransform(function func(string) string, input string) string {
	result := function(input)
	registerTransformedString(result, input)
	return result
}

func registerTransformedString(result, input string) {
	if ranges := conservativeRanges(len(result), RangesString(input)); len(ranges) > 0 {
		registerString(result, ranges)
	}
}
