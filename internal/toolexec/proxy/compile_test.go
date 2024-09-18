// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package proxy

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseCompile(t *testing.T) {
	for name, tc := range map[string]struct {
		input   []string
		goFiles []string
		flags   compileFlagSet
	}{
		"version_print": {
			input: []string{"/path/compile", "-V=full"},
			flags: compileFlagSet{
				ShowVersion: true,
			},
		},
		"compile": {
			input:   []string{"/path/compile", "-o", "/buildDir/b002/a.out", "-p", "mypackage", "-lang=go1.42", "-importcfg", "/buildDir/b002/importcfg", "/source/dir/main.go", "/source/dir/file1.go"},
			goFiles: []string{"/source/dir/main.go", "/source/dir/file1.go"},
			flags: compileFlagSet{
				Package:   "mypackage",
				ImportCfg: "/buildDir/b002/importcfg",
				Output:    "/buildDir/b002/a.out",
				Lang:      "go1.42",
			},
		},
		"nats.go": {
			input:   []string{"/path/compile", "-o", "/buildDir/b002/a.out", "-p", "github.com/nats-io/nats.go", "-complete", "/path/to/source/file.go"},
			goFiles: []string{"/path/to/source/file.go"},
			flags: compileFlagSet{
				Package: "github.com/nats-io/nats.go",
				Output:  "/buildDir/b002/a.out",
			},
		},
	} {
		if tc.goFiles == nil {
			// Simplify comparisons, as goFiles always returns non-nil
			tc.goFiles = make([]string, 0)
		}

		if name != "nats.go" {
			continue
		}
		t.Run(name, func(t *testing.T) {
			cmd, err := parseCompileCommand(tc.input)
			require.NoError(t, err)
			require.Equal(t, CommandTypeCompile, cmd.Type())
			c := cmd.(*CompileCommand)
			require.Equal(t, tc.flags, c.Flags)
			require.EqualValues(t, tc.goFiles, c.GoFiles())
		})
	}
}
