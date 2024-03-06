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
// otherwise ignored by the compiler (directives must begin at the first column of a line). It modifies the input slice
// to avoid re-allocating (since the output is guaranteed to be the same size or smaller than the input).
func postProcess(src []byte) []byte {
	for i := 0; i < len(src); {
		slice := src[i:]

		trimmed := bytes.TrimLeft(slice, " \t")
		trimmedLen := len(trimmed)
		if trimmedLen != len(slice) && bytes.HasPrefix(trimmed, lineDirectivePrefix) {
			// This line has a line directive with leading white space. We need to remove that whilte space, otherwise the
			// directive will be ignored by the compiler. To do so we move the source data left by the padding amount, and
			// then truncate the source slice to its new length.
			newEnd := i + trimmedLen
			copy(src[i:newEnd], trimmed)
			src = src[:newEnd]
		}

		// Advance to the next line
		lf := bytes.IndexByte(src[i:], '\n')
		if lf < 0 {
			// This was the last line: we can break out.
			break
		}
		i += lf + 1
	}

	return src
}
