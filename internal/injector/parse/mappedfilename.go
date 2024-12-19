// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package parse

import (
	"bytes"
	"io"
)

// consumeLineDirective consumes the first line from r if it's a "//line"
// directive that either does not have line/column information or has it set to
// line 1 (and column 1). If the directive is consumed, the filename it refers
// to is returned. Otherwise, the reader is rewinded to its original position.
func consumeLineDirective(r io.ReadSeeker) (string, error) {
	var buf [7]byte
	n, err := r.Read(buf[:])
	if err != nil {
		return "", err
	}
	if string(buf[:n]) != "//line " {
		_, err := r.Seek(0, io.SeekStart)
		return "", err
	}

	buffer := make([]byte, 0, 128)
	var wasCR, done bool
	for !done {
		if n, err := r.Read(buf[:1]); err != nil {
			return "", err
		} else if n == 0 {
			// Reached EOF
			break
		}
		switch c := buf[0]; c {
		case '\n':
			done = true
		case '\r':
			wasCR = true
			continue
		default:
			if wasCR {
				// We saw a CR and this is not an LF, so we rewind one byte and bail out.
				if _, err := r.Seek(-1, io.SeekCurrent); err != nil {
					return "", err
				}
				done = true
			} else {
				buffer = append(buffer, c)
			}
		}
	}

	// Remove any leading or trailing white space
	if rest, pos, hadPos := cutPositionSuffix(bytes.TrimSpace(buffer)); !hadPos {
		return string(rest), nil
	} else if pos != 1 {
		// It was not at position 1, so it's not the directive we're looking for.
		_, err := r.Seek(0, io.SeekStart)
		return "", err
	} else if rest, pos, hadPos := cutPositionSuffix(rest); !hadPos || pos == 1 {
		return string(rest), nil
	}
	// It was not at position 1, so it's not the directive we're looking for.
	_, err = r.Seek(0, io.SeekStart)
	return "", err
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
