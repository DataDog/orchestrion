// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package quoted provides string manipulation utilities. This is elements
// copied from the go command's internals:
// https://github.com/golang/go/blob/go1.23rc2/src/cmd/go/internal/base/goflags.go
package quoted

import (
	"fmt"
	"strings"
)

func isSpaceByte(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r'
}

// Split splits s into a list of fields,
// allowing single or double quotes around elements.
// There is no unescaping or other processing within
// quoted fields.
//
// Keep in sync with cmd/dist/quoted.go
func Split(s string) ([]string, error) {
	// Split fields allowing '' or "" around elements.
	// Quotes further inside the string do not count.
	var f []string
	for len(s) > 0 {
		for len(s) > 0 && isSpaceByte(s[0]) {
			s = s[1:]
		}
		if len(s) == 0 {
			break
		}
		// Accepted quoted string. No unescaping inside.
		if s[0] == '"' || s[0] == '\'' {
			quote := s[0]
			s = s[1:]
			i := 0
			for i < len(s) && s[i] != quote {
				i++
			}
			if i >= len(s) {
				return nil, fmt.Errorf("unterminated %c string", quote)
			}
			f = append(f, s[:i])
			s = s[i+1:]
			continue
		}
		i := 0
		for i < len(s) && !isSpaceByte(s[i]) {
			i++
		}
		f = append(f, s[:i])
		s = s[i:]
	}
	return f, nil
}

// Join joins a list of fields into a single string that can be parsed with [Split]. Fields that
// contain spaces or quotes are wrapped in single quotes (or double quotes if the field contains a
// single quote, mirroring [Split]'s acceptance rules; there is no escaping, as [Split] performs no
// unescaping). It returns an error if a field contains both kinds of quotes, in which case no
// valid quoting exists.
func Join(fields []string) (string, error) {
	var buf []byte
	for i, field := range fields {
		if i > 0 {
			buf = append(buf, ' ')
		}

		if strings.ContainsAny(field, " \t\n\r'\"") {
			var quote byte
			switch {
			case !strings.ContainsRune(field, '\''):
				quote = '\''
			case !strings.ContainsRune(field, '"'):
				quote = '"'
			default:
				return "", fmt.Errorf("field %q contains both single and double quotes and cannot be quoted", field)
			}
			buf = append(buf, quote)
			buf = append(buf, field...)
			buf = append(buf, quote)
			continue
		}

		buf = append(buf, field...)
	}
	return string(buf), nil
}
