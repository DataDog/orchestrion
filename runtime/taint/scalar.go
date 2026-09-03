// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package taint

import "unsafe"

// MarkByteFromString associates an extracted string byte with its local occurrence.
func MarkByteFromString[B ~byte, S ~string](target *B, source S, index int) {
	active.setByteScalarTaint(scalarAddress(target), rangesContain(RangesString(string(source)), index))
}

// MarkByteFromBytes associates an extracted slice byte with its local occurrence.
func MarkByteFromBytes[B ~byte, S ~[]E, E ~byte](target *B, source S, index int) {
	active.setByteScalarTaint(scalarAddress(target), rangesContain(RangesBytes(source), index))
}

// ReleaseByte removes metadata for a local byte occurrence at function exit.
func ReleaseByte[B ~byte](target *B) {
	active.releaseByteScalar(scalarAddress(target))
}

// ByteScalarToStringAs converts a local byte occurrence into a string type.
func ByteScalarToStringAs[T ~string, B ~byte](source *B) T {
	result := T(isolatedString(string(rune(*source))))
	if active.byteScalarTainted(scalarAddress(source)) {
		registerString(string(result), []Range{{Start: 0, End: len(result)}})
	}
	return result
}

// MarkRuneFromRunes associates an extracted rune with its local occurrence.
func MarkRuneFromRunes[R ~rune, S ~[]E, E ~rune](target *R, source S, index int) {
	active.setRuneScalarTaint(scalarAddress(target), rangesContain(RangesRunes(source), index))
}

// ReleaseRune removes metadata for a local rune occurrence at function exit.
func ReleaseRune[R ~rune](target *R) {
	active.releaseRuneScalar(scalarAddress(target))
}

// RuneScalarToStringAs converts a local rune occurrence into a string type.
func RuneScalarToStringAs[T ~string, R ~rune](source *R) T {
	result := T(isolatedString(string(rune(*source))))
	if active.runeScalarTainted(scalarAddress(source)) {
		registerString(string(result), []Range{{Start: 0, End: len(result)}})
	}
	return result
}

func scalarAddress[T any](value *T) unsafe.Pointer {
	return unsafe.Pointer(value)
}
