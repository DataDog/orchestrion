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

func TestParseAsm(t *testing.T) {
	for name, tc := range map[string]struct {
		input    []string
		stage    string
		stageDir string
		flags    asmFlagSet
	}{
		"version_print": {
			input:    []string{"/path/asm", "-V=full"},
			stage:    ".",
			stageDir: "",
		},
		"asm": {
			input:    []string{"/path/asm", "-o", "/buildDir/b002/file1.o", "-p", "mypackage", "/srcDir/file1.s"},
			stage:    "b002",
			stageDir: "/buildDir/b002",
			flags: asmFlagSet{
				Package: "mypackage",
				Output:  "/buildDir/b002/file1.o",
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			cmd, err := parseAsmCommand(tc.input)
			require.NoError(t, err)
			require.Equal(t, CommandTypeAsm, cmd.Type())
			require.Equal(t, tc.stage, cmd.Stage())
			a := cmd.(*AsmCommand)
			require.True(t, reflect.DeepEqual(tc.flags, a.Flags))
			require.Equal(t, tc.stageDir, a.StageDir)
		})
	}
}
