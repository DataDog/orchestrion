// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package goflags

import (
	"context"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
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
			flags := tc.flags.Except(tc.remove...)
			for _, flag := range tc.remove {
				require.NotContains(t, flags.Long, flag)
				require.NotContains(t, flags.Short, flag)
			}
		})
	}
}

func TestParse(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	thisDir := filepath.Dir(thisFile)

	for name, tc := range map[string]struct {
		flags    []string
		goflags  string
		expected CommandFlags
		// If true, do not override the shortFlags and logFlags internals, instead use the standard values.
		useStdFlags bool
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
				Long:  map[string]string{"-covermode": "atomic", "-coverpkg": "github.com/DataDog/orchestrion/internal/goflags"},
				Short: map[string]struct{}{"-cover": {}},
			},
		},
		"covermode": {
			flags: []string{"run", "-covermode=count"},
			expected: CommandFlags{
				Long:  map[string]string{"-covermode": "count", "-coverpkg": "github.com/DataDog/orchestrion/internal/goflags"},
				Short: nil,
			},
		},
		"cover-with-coverpkg": {
			flags:   []string{"run", "-cover", "-covermode=atomic", "--", "-some.go"},
			goflags: "-coverpkg=std,./...",
			expected: CommandFlags{
				Long:  map[string]string{"-covermode": "atomic", "-coverpkg": "std,github.com/DataDog/orchestrion/internal/goflags,github.com/DataDog/orchestrion/internal/goflags/quoted"},
				Short: map[string]struct{}{"-cover": {}},
			},
		},
		"cover-dash-c": {
			flags: []string{"-C", "..", "run", "-cover", "-covermode=atomic"},
			expected: CommandFlags{
				// Note - the "-C" flags has no effect at this stage, so it's expected coverpkg is this package.
				Long:  map[string]string{"-covermode": "atomic", "-coverpkg": "github.com/DataDog/orchestrion/internal/goflags"},
				Short: map[string]struct{}{"-cover": {}},
			},
		},
		"cover-dash-c-alt": {
			flags: []string{"-C=..", "run", "-cover", "-covermode=atomic", "."},
			expected: CommandFlags{
				// Note - the "-C" flags has no effect at this stage, so it's expected coverpkg is this package.
				Long:  map[string]string{"-covermode": "atomic", "-coverpkg": "github.com/DataDog/orchestrion/internal/goflags"},
				Short: map[string]struct{}{"-cover": {}},
			},
		},
		"goflags": {
			flags:   []string{"run", "."},
			goflags: "-cover -covermode=atomic -tags=integration '-toolexec=foo bar'",
			expected: CommandFlags{
				Long:  map[string]string{"-covermode": "atomic", "-coverpkg": "github.com/DataDog/orchestrion/internal/goflags", "-tags": "integration", "-toolexec": "foo bar"},
				Short: map[string]struct{}{"-cover": {}},
			},
		},
		"coverpkg-relative-with-goflags": {
			flags:   []string{"test", "./...", "-timeout", "30m", "-cover", "-covermode=atomic", "-coverprofile=coverage.out", "-coverpkg", "./..."},
			goflags: `"-toolexec=orchestrion toolexec"`,
			expected: CommandFlags{
				Long: map[string]string{
					"-covermode": "atomic",
					"-coverpkg":  "github.com/DataDog/orchestrion/internal/goflags,github.com/DataDog/orchestrion/internal/goflags/quoted",
					"-toolexec":  "orchestrion toolexec",
				},
				Short:   map[string]struct{}{"-cover": {}},
				Unknown: []string{"-timeout", "30m", "-coverprofile=coverage.out"},
			},
			useStdFlags: true,
		},
	} {
		// Make sure the expected outcomes are non-nil, makes it easier to validate afterwards.
		if tc.expected.Short == nil {
			tc.expected.Short = make(map[string]struct{})
		}
		if tc.expected.Long == nil {
			tc.expected.Long = make(map[string]string)
		}

		t.Run(name, func(t *testing.T) {
			if !tc.useStdFlags {
				defer restore(shortFlags, longFlags)
				shortFlags = tc.expected.Short
				longFlags = make(map[string]struct{}, len(tc.expected.Long))
				for flag := range tc.expected.Long {
					longFlags[flag] = struct{}{}
				}
			}

			t.Setenv("GOFLAGS", tc.goflags)
			flags, err := ParseCommandFlags(context.Background(), thisDir, tc.flags)
			require.NoError(t, err)

			if flags.Short == nil {
				flags.Short = make(map[string]struct{})
			}
			assert.True(t, reflect.DeepEqual(tc.expected.Short, flags.Short), "expected:\n%#v\nactual:\n%#v", tc.expected.Short, flags.Short)

			if flags.Long == nil {
				flags.Long = make(map[string]string)
			}
			assert.True(t, reflect.DeepEqual(tc.expected.Long, flags.Long), "expected:\n%#v\nactual:\n%#v", tc.expected.Long, flags.Long)
		})
	}
}

func restore(short map[string]struct{}, long map[string]struct{}) {
	shortFlags = short
	longFlags = long
}
