// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package injector

import (
	"bytes"
)

var lineDirectivePrefix = []byte("//line ")

// postProcess modifies the provided source text to remove leading white space ahead of //line directives, as they are
// otherwise ignored by the compiler (directives must begin at the first column of a line). It also removes redundant
// line directives, keeping only the last of a streak of multiple directives.
//
// It modifies the input slice in-place to avoid re-allocating (since the output is guaranteed to be the same size or
// smaller than the input).
func postProcess(src []byte) []byte {
	lastDirectiveIndex := -1

	for i := 0; i < len(src); i += nextLineStart(src[i:]) {
		slice := src[i:]

		trimmed := bytes.TrimLeft(slice, " \t")
		if !bytes.HasPrefix(trimmed, lineDirectivePrefix) {
			lastDirectiveIndex = -1
			continue
		}

		trimmedLen := len(trimmed)
		if lastDirectiveIndex >= 0 {
			// We are in a line directive streak, so we will move this directive over the previous one we identified, so that
			// only one is left.
			newEnd := lastDirectiveIndex + trimmedLen
			copy(src[lastDirectiveIndex:newEnd], trimmed)
			src = src[:newEnd]
			// Roll the pointer back to the lastDirectiveIndex to resume from there
			i = lastDirectiveIndex
			continue
		}

		lastDirectiveIndex = i
		if trimmedLen != len(slice) {
			// This line has a line directive with leading white space. We need to remove that whilte space, otherwise the
			// directive will be ignored by the compiler. To do so we move the source data left by the padding amount, and
			// then truncate the source slice to its new length.
			newEnd := i + trimmedLen
			copy(src[i:newEnd], trimmed)
			src = src[:newEnd]
		}
	}

	return src
}

// nextLineStart returns the index of the first byte right after a CR, LF or CR+LF sequence in the provided slice. If
// none of these sequences are found, it returns the length of the slice.
func nextLineStart(slice []byte) int {
	cr := bytes.IndexByte(slice, '\r')
	lf := bytes.IndexByte(slice, '\n')
	if cr < 0 && lf < 0 {
		return len(slice)
	}
	if lf >= 0 {
		return lf + 1
	}
	return cr + 1
}
