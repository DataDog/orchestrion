// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

//go:build windows

package goflags

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCommandLineToArgv(t *testing.T) {
	type testCase struct {
		input          string
		expectedOutput []string
	}

	testCases := map[string]testCase{
		"quoted": {
			input:          `"C:\hostedtoolcache\windows\go\1.23.4\x64\bin\go.exe" go test ./...`,
			expectedOutput: []string{`C:\hostedtoolcache\windows\go\1.23.4\x64\bin\go.exe`, "go", "test", "./..."},
		},
		"spaces": {
			input:          `"C:\Program Files\go.exe" go test ./...`,
			expectedOutput: []string{`C:\Program Files\go.exe`, "go", "test", "./..."},
		},
		"missing-end-quote": {
			// You'd have expected this to result in an error? You'd be wrong. Windows' CommandLineToArgvW
			// function is extremely lax and will happily parse this as if there was an end quote at the
			// end of the input... Yay...?
			input:          `"C:\Program Files\go.exe go test ./...`,
			expectedOutput: []string{`C:\Program Files\go.exe go test ./...`},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			actual, err := commandLineToArgv(tc.input)
			require.NoError(t, err)
			require.Equal(t, tc.expectedOutput, actual)
		})
	}
}
