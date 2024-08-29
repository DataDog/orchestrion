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

type testFlagSet struct {
	FlagStr  string `ddflag:"-flagStr"`
	FlagBool bool   `ddflag:"-flagBool"`
}

func TestParseFlags(t *testing.T) {
	for name, tc := range map[string]struct {
		args     []string
		expected testFlagSet
		panic    bool
	}{
		"flagStr/plain": {
			args: []string{"-flagStr", "test"},
			expected: testFlagSet{
				FlagStr: "test",
			},
		},
		"flagStr/assignment": {
			args: []string{"-flagStr=test"},
			expected: testFlagSet{
				FlagStr: "test",
			},
		},
		"flagStr/assignment-empty": {
			args: []string{"-flagStr="},
		},
		"flagBool": {
			args: []string{"-flagBool"},
			expected: testFlagSet{
				FlagBool: true,
			},
		},
		"combined/1": {
			args: []string{"-flagStr", "test", "-flagBool"},
			expected: testFlagSet{
				FlagStr:  "test",
				FlagBool: true,
			},
		},
		"combined/2": {
			args: []string{"-flagStr=test", "-flagBool"},
			expected: testFlagSet{
				FlagStr:  "test",
				FlagBool: true,
			},
		},
		"combined/3": {
			args: []string{"-flagBool", "-flagStr", "test"},
			expected: testFlagSet{
				FlagStr:  "test",
				FlagBool: true,
			},
		},
		"invalid/flagStr/1": {
			args:  []string{"-flagStr", "-flagBool"},
			panic: true,
		},
		"invalid/flagStr/2": {
			args:  []string{"-flagStr"},
			panic: true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			defer func() {
				r := recover()
				require.Equal(t, tc.panic, r != nil)
			}()
			flags := testFlagSet{}
			parseFlags(&flags, tc.args)
			require.True(t, reflect.DeepEqual(tc.expected, flags))
		})
	}
}
