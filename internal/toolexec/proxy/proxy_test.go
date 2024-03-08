// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package proxy_test

import (
	"reflect"
	"testing"

	"github.com/datadog/orchestrion/internal/toolexec/proxy"

	"github.com/stretchr/testify/require"
)

func TestReplaceParam(t *testing.T) {
	for name, tc := range map[string]struct {
		params []string
		old    string
		new    string
		error  bool
	}{
		"found": {
			params: []string{"compile", "-o", "a.out"},
			old:    "a.out",
			new:    "b.out",
		},
		"not-found": {
			params: []string{"compile", "-o", "a.out"},
			old:    "b.out",
			new:    "c.out",
			error:  true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			cmd := proxy.NewCommand(tc.params)
			require.Equal(t, tc.params, cmd.Args())
			require.NotContains(t, cmd.Args(), tc.new)
			err := cmd.ReplaceParam(tc.old, tc.new)
			if tc.error {
				require.NotNil(t, err)
			} else {
				require.NoError(t, err)
				require.Contains(t, cmd.Args(), tc.new)
				require.NotContains(t, cmd.Args(), tc.old)
			}
		})
	}
}

func TestParseCommand(t *testing.T) {
	for name, tc := range map[string]struct {
		input         []string
		expectedType  proxy.CommandType
		expectedStage string
	}{
		"unknown": {
			input:         []string{"unknown", "irrelevant"},
			expectedType:  proxy.CommandTypeOther,
			expectedStage: ".",
		},
		"compile": {
			input:         []string{"compile", "-o", "b002/a.out", "main.go"},
			expectedType:  proxy.CommandTypeCompile,
			expectedStage: "b002",
		},
		"link": {
			input:         []string{"link", "-o", "b001/out/a.out", "main.go"},
			expectedType:  proxy.CommandTypeLink,
			expectedStage: "b001",
		},
	} {

		t.Run(name, func(t *testing.T) {
			cmd := proxy.MustParseCommand(tc.input)
			require.Equal(t, tc.expectedType, cmd.Type())
			require.Equal(t, tc.expectedStage, cmd.Stage())
			require.True(t, reflect.DeepEqual(tc.input, cmd.Args()))
		})
	}
}
