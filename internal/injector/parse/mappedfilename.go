// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package parse

import (
	"bytes"
)

// consumeLineDirective consumes the first line from r if it's a "//line"
// directive that either does not have line/column information or has it set to
// line 1 (and column 1). If the directive is consumed, the filename it refers
// to is returned. Otherwise, the reader is rewinded to its original position.
func consumeLineDirective(d []byte, filename string) (string, []byte, error) {
	if len(d) < 7 {
		// There cannot be a "//line " directive here, since there's not enough data.
		return filename, d, nil
	}
	if string(d[:7]) != "//line " {
		// There is no "//line " directive at the start of the file, so we just
		// return the original filename and full data.
		return filename, d, nil
	}

	si := 7 // Value start index
	for si < len(d) && d[si] == ' ' {
		si++
	}

	ei := si // Value end index (exclusive)
	ds := ei // Data start index
	for ; ei < len(d); ei++ {
		if d[ei] == '\r' {
			if ei+1 < len(d) && d[ei+1] == '\n' {
				// CRLF
				ds = ei + 2
			} else {
				// CR
				ds = ei + 1
			}
			break
		}
		if d[ei] == '\n' {
			ds = ei + 1
			break
		}
		ds = ei
	}

	dv := bytes.TrimSpace(d[si:ei])
	fn, pos, ok := cutPositionSuffix(dv)
	if !ok {
		// There is no position suffix, so we just remove the directive and return
		// its value as-is...
		return string(fn), d[ds:], nil
	}
	if ok && pos != 1 {
		// We only trim if the position is 1, otherwise this isn't the directive
		// we're looking to trim.
		return filename, d, nil
	}
	fn, pos, ok = cutPositionSuffix(fn)
	if ok && pos != 1 {
		// We only trim if the position is 1, otherwise this isn't the directive
		// we're looking to trim.
		return filename, d, nil
	}

	return string(fn), d[ds:], nil
}

// cutPositionSuffix removes a trailing ":<int>" from the provided buffer, if present.
func cutPositionSuffix(buf []byte) ([]byte, int, bool) {
	cutOff := len(buf) - 1

	// First, consume the integer at the end of the buffer.
	pos := 0
	pow := 1
	for buf[cutOff] >= '0' && buf[cutOff] <= '9' {
		pos += pow * int(buf[cutOff]-'0')
		pow *= 10
		cutOff--
	}

	// If there's no ":" before the integer, or there was no digit at all, it was not a position...
	if buf[cutOff] != ':' || pow == 1 {
		return buf, 0, false
	}

	return buf[:cutOff], pos, true
}
