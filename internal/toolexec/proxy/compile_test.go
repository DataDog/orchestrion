// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package proxy

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseCompile(t *testing.T) {
	for name, tc := range map[string]struct {
		input    []string
		stage    string
		buildDir string
		goFiles  []string
		flags    compileFlagSet
	}{
		"version_print": {
			input: []string{"/path/compile", "-V=full"},
			stage: ".",
		},
		"compile": {
			input:    []string{"/path/compile", "-o", "/buildDir/b002/a.out", "-p", "mypackage", "-importcfg", "/buildDir/b002/importcfg", "/buildDir/b002/main.go", "/buildDir/b002/file1.go"},
			stage:    "b002",
			buildDir: "/buildDir/b002",
			goFiles:  []string{"/buildDir/b002/main.go", "/buildDir/b002/file1.go"},
			flags: compileFlagSet{
				Package:   "mypackage",
				ImportCfg: "/buildDir/b002/importcfg",
				Output:    "/buildDir/b002/a.out",
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			cmd, err := parseCompileCommand(tc.input)
			require.NoError(t, err)
			require.Equal(t, CommandTypeCompile, cmd.Type())
			require.Equal(t, tc.stage, cmd.Stage())
			c := cmd.(*CompileCommand)
			require.True(t, reflect.DeepEqual(tc.flags, c.Flags))
			require.Equal(t, tc.buildDir, c.BuildDir)
		})
	}
}
