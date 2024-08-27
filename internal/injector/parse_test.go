// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package injector

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConsumeLineDirective(t *testing.T) {
	source := []string{
		"package main",
		"",
		"func main() {",
		"\t/* ... */",
		"}",
		"",
	}

	lineSeparators := map[string]string{
		"CR":   "\r",
		"LF":   "\n",
		"CRLF": "\r\n",
	}

	for sepStyle, sep := range lineSeparators {
		t.Run(sepStyle, func(t *testing.T) {
			sourceBytes := []byte(strings.Join(source, sep))

			cases := map[string]string{
				"// no directive": "",
				// Bad directives (ignored)
				"//line path/to/file.go:1:42": "",
				"//line path/to/file.go:1337": "",
				// Valid directives
				"//line path/to/file.go:1:1": "path/to/file.go",
				"//line path/to/file.go:1":   "path/to/file.go",
				"//line path/to/file.go":     "path/to/file.go",
			}
			for directive, expectedOutcome := range cases {
				t.Run(directive, func(t *testing.T) {
					var buffer bytes.Buffer
					buffer.WriteString(directive)
					buffer.WriteString(sep)
					buffer.Write(sourceBytes)

					reader := bytes.NewReader(buffer.Bytes())
					filename, err := consumeLineDirective(reader)
					require.NoError(t, err)
					require.Equal(t, expectedOutcome, filename)

					rest, err := io.ReadAll(reader)
					require.NoError(t, err)

					expected := sourceBytes
					if expectedOutcome == "" {
						expected = buffer.Bytes()
					}

					require.Equal(t, string(expected), string(rest))
				})
			}
		})
	}
}
