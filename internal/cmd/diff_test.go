// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package cmd_test

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/DataDog/orchestrion/internal/cmd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrepareBuildArgs(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "empty args defaults to build with ./...",
			input:    nil,
			expected: []string{"build", "-work", "-a", "./..."},
		},
		{
			name:     "build command without flags",
			input:    []string{"build", "main.go"},
			expected: []string{"build", "-work", "-a", "main.go"},
		},
		{
			name:     "install command without flags",
			input:    []string{"install", "./..."},
			expected: []string{"install", "-work", "-a", "./..."},
		},
		{
			name:     "test command without flags",
			input:    []string{"test", "./..."},
			expected: []string{"test", "-work", "-a", "./..."},
		},
		{
			name:     "non-build command gets build prepended",
			input:    []string{"./main.go"},
			expected: []string{"build", "-work", "-a", "./main.go"},
		},
		{
			name:     "with existing -work flag",
			input:    []string{"build", "-work", "main.go"},
			expected: []string{"build", "-a", "-work", "main.go"},
		},
		{
			name:     "with existing -a flag",
			input:    []string{"build", "-a", "main.go"},
			expected: []string{"build", "-work", "-a", "main.go"},
		},
		{
			name:     "with both -work and -a flags",
			input:    []string{"build", "-work", "-a", "main.go"},
			expected: []string{"build", "-work", "-a", "main.go"},
		},
		{
			name:     "complex build with other flags",
			input:    []string{"build", "-v", "-x", "main.go"},
			expected: []string{"build", "-work", "-a", "-v", "-x", "main.go"},
		},
		{
			name:     "install with mixed flags",
			input:    []string{"install", "-work", "-v", "./..."},
			expected: []string{"install", "-a", "-work", "-v", "./..."},
		},
		{
			name:     "test with all flags already present",
			input:    []string{"test", "-work", "-a", "-v", "./..."},
			expected: []string{"test", "-work", "-a", "-v", "./..."},
		},
		{
			name:     "args with empty strings",
			input:    []string{"build", "", "-v", ""},
			expected: []string{"build", "-work", "-a", "", "-v", ""},
		},
		{
			name:     "only flags without targets",
			input:    []string{"build", "-v", "-x"},
			expected: []string{"build", "-work", "-a", "-v", "-x"},
		},
		{
			name:     "mixed case command (build in caps)",
			input:    []string{"BUILD", "main.go"},
			expected: []string{"build", "-work", "-a", "BUILD", "main.go"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use internal access to test the function
			result := cmd.PrepareBuildArgsForTest(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractWorkDirFromOutput(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		expected string
	}{
		{
			name:     "valid work directory output",
			output:   "WORK=/tmp/go-build123456789",
			expected: "/tmp/go-build123456789",
		},
		{
			name: "work directory with other output",
			output: `some output
WORK=/tmp/go-build987654321
more output`,
			expected: "/tmp/go-build987654321",
		},
		{
			name: "work directory with whitespace",
			output: `
			WORK=/tmp/go-build111222333
			`,
			expected: "/tmp/go-build111222333",
		},
		{
			name:     "no work directory in output",
			output:   "some build output without work directory",
			expected: "",
		},
		{
			name:     "empty output",
			output:   "",
			expected: "",
		},
		{
			name:     "work directory with spaces in path",
			output:   "WORK=/tmp/go build/spaces in path",
			expected: "/tmp/go build/spaces in path",
		},
		{
			name: "multiple work directories - returns first",
			output: `WORK=/tmp/first
WORK=/tmp/second`,
			expected: "/tmp/first",
		},
		{
			name:     "work directory with equals in path",
			output:   "WORK=/tmp/go=build=123",
			expected: "/tmp/go=build=123",
		},
		{
			name:     "false positive - WORK in the middle of line",
			output:   "some text WORK=/tmp/not-real",
			expected: "",
		},
		{
			name:     "empty WORK value",
			output:   "WORK=",
			expected: "",
		},
		{
			name: "WORK with newline immediately after equals",
			output: `WORK=
/tmp/go-build123`,
			expected: "",
		},
		{
			name:     "WORK with only whitespace after equals",
			output:   "WORK=   \t  ",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cmd.ExtractWorkDirFromOutputForTest(tt.output)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Mock report for testing
type mockReport struct {
	packages []string
	files    []string
	isEmpty  bool
	diffErr  error
}

func (m *mockReport) IsEmpty() bool {
	return m.isEmpty
}

func (m *mockReport) WithSpecialCasesFilter() cmd.ReportInterface {
	return m
}

func (m *mockReport) WithRegexFilter(pattern string) (cmd.ReportInterface, error) {
	if pattern == "invalid[" {
		return nil, errors.New("invalid regex pattern")
	}
	return m, nil
}

func (m *mockReport) Packages() []string {
	return m.packages
}

func (m *mockReport) Files() []string {
	return m.files
}

func (m *mockReport) Diff(w io.Writer) error {
	if m.diffErr != nil {
		return m.diffErr
	}
	_, err := w.Write([]byte("mock diff output"))
	return err
}

func TestOutputReport(t *testing.T) {
	tests := []struct {
		name           string
		packageFlag    bool
		filenameFlag   bool
		report         *mockReport
		expectedOutput string
		expectedError  string
	}{
		{
			name:        "package flag enabled",
			packageFlag: true,
			report: &mockReport{
				packages: []string{"pkg1", "pkg2", "pkg3"},
			},
			expectedOutput: "pkg1\npkg2\npkg3\n",
		},
		{
			name:         "filename flag enabled",
			filenameFlag: true,
			report: &mockReport{
				files: []string{"file1.go", "file2.go"},
			},
			expectedOutput: "file1.go\nfile2.go\n",
		},
		{
			name: "diff output",
			report: &mockReport{
				packages: []string{"pkg1"},
				files:    []string{"file1.go"},
			},
			expectedOutput: "mock diff output",
		},
		{
			name: "diff error",
			report: &mockReport{
				diffErr: errors.New("diff generation failed"),
			},
			expectedError: "failed to generate diff: diff generation failed",
		},
		{
			name:        "package flag with nil packages",
			packageFlag: true,
			report: &mockReport{
				packages: nil,
			},
			expectedOutput: "",
		},
		{
			name:         "filename flag with nil files",
			filenameFlag: true,
			report: &mockReport{
				files: nil,
			},
			expectedOutput: "",
		},
		{
			name: "empty report with no flags",
			report: &mockReport{
				packages: nil,
				files:    nil,
			},
			expectedOutput: "mock diff output",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var output bytes.Buffer

			flags := make(map[string]bool)
			if tt.packageFlag {
				flags["package"] = true
			}
			if tt.filenameFlag {
				flags["files"] = true
			}

			err := cmd.OutputReportForTest(&output, flags, tt.report)

			if tt.expectedError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedOutput, output.String())
			}
		})
	}
}

func TestOutputReport_PriorityFlags(t *testing.T) {
	// Test that package flag takes priority over filename flag
	var output bytes.Buffer
	report := &mockReport{
		packages: []string{"pkg1"},
		files:    []string{"file1.go"},
	}

	flags := map[string]bool{
		"package": true,
		"files":   true,
	}

	err := cmd.OutputReportForTest(&output, flags, report)
	require.NoError(t, err)

	// Should output packages, not files
	assert.Equal(t, "pkg1\n", output.String())
}

func TestOutputReport_EmptyResults(t *testing.T) {
	tests := []struct {
		name         string
		packageFlag  bool
		filenameFlag bool
		report       *mockReport
	}{
		{
			name:        "empty packages",
			packageFlag: true,
			report:      &mockReport{packages: nil},
		},
		{
			name:         "empty files",
			filenameFlag: true,
			report:       &mockReport{files: nil},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var output bytes.Buffer

			flags := make(map[string]bool)
			if tt.packageFlag {
				flags["package"] = true
			}
			if tt.filenameFlag {
				flags["files"] = true
			}

			err := cmd.OutputReportForTest(&output, flags, tt.report)
			require.NoError(t, err)
			assert.Empty(t, output.String())
		})
	}
}
