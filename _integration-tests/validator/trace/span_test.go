// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package trace

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"unicode"

	"github.com/stretchr/testify/require"
	"gotest.tools/v3/golden"
)

var testdata string

func TestMatchesAny(t *testing.T) {
	cases, err := os.ReadDir(testdata)
	require.NoError(t, err)
	for _, caseDir := range cases {
		if !caseDir.IsDir() {
			continue
		}

		name := caseDir.Name()
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var (
				expected *Span
				actual   []*Span
			)
			{
				data, err := os.ReadFile(filepath.Join(testdata, name, "expected.json"))
				require.NoError(t, err)
				require.NoError(t, json.Unmarshal(data, &expected))
			}
			{
				data, err := os.ReadFile(filepath.Join(testdata, name, "actual.json"))
				require.NoError(t, err)
				require.NoError(t, json.Unmarshal(data, &actual))
			}

			matches, diff := expected.MatchesAny(actual)
			goldFile := filepath.Join(name, "diff.txt")
			if matches {
				golden.Assert(t, "<none>", goldFile)
				require.Empty(t, diff, 0)
			} else {
				require.NotEmpty(t, diff)
				golden.Assert(t, strings.TrimRightFunc(diff.String(), unicode.IsSpace), goldFile)
			}
		})
	}
}

func renderDiff(diff []Diff) []byte {
	var buf bytes.Buffer

	for idx, entry := range diff {
		if idx > 0 {
			buf.WriteByte('\n')
		}
		buf.WriteString(strings.Repeat("#", 72))
		buf.WriteString(fmt.Sprintf("\n// At index %d\n", idx))
		buf.WriteString(entry.String())
		buf.WriteByte('\n')
	}

	return buf.Bytes()
}

func init() {
	_, file, _, _ := runtime.Caller(0)
	testdata = filepath.Join(file, "..", "testdata")
}
