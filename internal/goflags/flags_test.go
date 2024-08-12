// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package goflags

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTrim(t *testing.T) {
	for name, tc := range map[string]struct {
		flags  CommandFlags
		remove []string
	}{
		"not found": {
			flags: CommandFlags{
				Long:  map[string]string{"-long1": "long1val"},
				Short: map[string]struct{}{"-short1": {}},
			},
			remove: []string{"-notfound"},
		},
		"found": {
			flags: CommandFlags{
				Long:  map[string]string{"-long1": "long1val", "-long2": "long2val"},
				Short: map[string]struct{}{"-short1": {}, "-short2": {}},
			},
			remove: []string{"-short1", "-long1"},
		},
	} {
		t.Run(name, func(t *testing.T) {
			tc.flags.Trim(tc.remove...)
			for _, flag := range tc.remove {
				require.NotContains(t, tc.flags.Long, flag)
				require.NotContains(t, tc.flags.Short, flag)
			}
		})

	}
}

func TestParse(t *testing.T) {
	for name, tc := range map[string]struct {
		flags    []string
		goflags  string
		expected CommandFlags
	}{
		"short": {
			flags:    []string{"run", "-short1", "--short2"},
			expected: CommandFlags{Short: map[string]struct{}{"-short1": {}, "-short2": {}}},
		},
		"long": {
			flags:    []string{"run", "-long1", "longval1", "--long2", "longval2"},
			expected: CommandFlags{Long: map[string]string{"-long1": "longval1", "-long2": "longval2"}},
		},
		"long-assigned": {
			flags:    []string{"run", "-long1=longval1", "--long2=longval2"},
			expected: CommandFlags{Long: map[string]string{"-long1": "longval1", "-long2": "longval2"}},
		},
		"long-mixed": {
			flags:    []string{"run", "-long1=longval1", "-long2", "longval2"},
			expected: CommandFlags{Long: map[string]string{"-long1": "longval1", "-long2": "longval2"}},
		},
		"special": {
			flags: []string{"run", "-gcflags", "-N -l -other", "-ldflags", "-extldflags '-lm -lstdc++ -static'"},
			expected: CommandFlags{
				Long: map[string]string{"-gcflags": "-N -l -other", "-ldflags": "-extldflags '-lm -lstdc++ -static'"},
			},
		},
		"combined": {
			flags: []string{"run", "-short1", "-gcflags", "-N -l -other", "-ldflags", "-extldflags '-lm -lstdc++ -static'", "-long1=longval1", "-short2", "-long2", "longval2"},
			expected: CommandFlags{
				Long:  map[string]string{"-gcflags": "-N -l -other", "-ldflags": "-extldflags '-lm -lstdc++ -static'", "-long1": "longval1", "-long2": "longval2"},
				Short: map[string]struct{}{"-short1": {}, "-short2": {}},
			},
		},
		"combined-and-unknown": {
			flags: []string{"run", "-unknown1", "-short1", "-long1=longval1", "-unknown2", "-short2", "-long2", "longval2", "unknown3"},
			expected: CommandFlags{
				Long:  map[string]string{"-long1": "longval1", "-long2": "longval2"},
				Short: map[string]struct{}{"-short1": {}, "-short2": {}},
			},
		},
		"cover": {
			flags: []string{"run", "-cover", "-covermode=atomic"},
			expected: CommandFlags{
				Long:  map[string]string{"-covermode": "atomic", "-coverpkg": "github.com/datadog/orchestrion/internal/goflags"},
				Short: map[string]struct{}{"-cover": {}},
			},
		},
		"cover-with-coverpkg": {
			flags:   []string{"run", "-cover", "-covermode=atomic", "--", "-some.go"},
			goflags: "-coverpkg=std",
			expected: CommandFlags{
				Long:  map[string]string{"-covermode": "atomic", "-coverpkg": "std"},
				Short: map[string]struct{}{"-cover": {}},
			},
		},
		"cover-dash-c": {
			flags: []string{"-C", "..", "run", "-cover", "-covermode=atomic", ".."},
			expected: CommandFlags{
				Long:  map[string]string{"-covermode": "atomic", "-coverpkg": "github.com/datadog/orchestrion"},
				Short: map[string]struct{}{"-cover": {}},
			},
		},
		"cover-dash-c-alt": {
			flags: []string{"-C=..", "run", "-cover", "-covermode=atomic", ".."},
			expected: CommandFlags{
				Long:  map[string]string{"-covermode": "atomic", "-coverpkg": "github.com/datadog/orchestrion"},
				Short: map[string]struct{}{"-cover": {}},
			},
		},
		"goflags": {
			flags:   []string{"run", "."},
			goflags: "-cover -covermode=atomic -tags=integration '-toolexec=foo bar'",
			expected: CommandFlags{
				Long:  map[string]string{"-covermode": "atomic", "-coverpkg": "github.com/datadog/orchestrion/internal/goflags", "-tags": "integration", "-toolexec": "foo bar"},
				Short: map[string]struct{}{"-cover": {}},
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			defer restore(shortFlags, longFlags)
			shortFlags = tc.expected.Short
			longFlags = map[string]struct{}{}
			for flag := range tc.expected.Long {
				longFlags[flag] = struct{}{}
			}

			flags := ParseCommandFlags("", tc.flags, tc.goflags)
			if len(tc.expected.Short) > 0 {
				require.True(t, reflect.DeepEqual(tc.expected.Short, flags.Short), "expected:\n%#v\nactual:\n%#v", tc.expected.Short, flags.Short)
			}
			if len(tc.expected.Long) > 0 {
				require.True(t, reflect.DeepEqual(tc.expected.Long, flags.Long), "expected:\n%#v\nactual:\n%#v", tc.expected.Long, flags.Long)
				for key, val := range tc.expected.Long {
					actual, _ := flags.Get(key)
					require.Equal(t, val, actual)
				}
			}
		})
	}
}

func restore(short map[string]struct{}, long map[string]struct{}) {
	shortFlags = short
	longFlags = long
}
