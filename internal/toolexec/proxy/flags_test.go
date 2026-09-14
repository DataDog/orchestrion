// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package proxy

import (
	"flag"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseToolFlags(t *testing.T) {
	for name, tc := range map[string]struct {
		args        []string
		expectPosit []string
		expectFoo   string // value captured for the known "-foo" string flag
		expectBar   bool   // value captured for the known "-bar" bool flag
		expectError string
	}{
		"known_flags_only": {
			args:        []string{"-foo", "foovalue", "-bar", "pos.go"},
			expectFoo:   "foovalue",
			expectBar:   true,
			expectPosit: []string{"pos.go"},
		},
		"unknown_flag_equals_form": {
			args:        []string{"-foo=known", "-exportfd=3", "pos.go"},
			expectFoo:   "known",
			expectPosit: []string{"pos.go"},
		},
		"unknown_flag_two_token_form": {
			// The next token does not look like a flag nor like an input file,
			// so it is assumed to be the unknown flag's value.
			args:        []string{"-exportfd", "3", "pos.go"},
			expectPosit: []string{"pos.go"},
		},
		"unknown_flag_value_looks_like_input_file": {
			// The next token looks like an input file: the unknown flag is
			// treated as value-less and the file stays positional.
			args:        []string{"-futureflag", "main.go", "pos.go"},
			expectPosit: []string{"main.go", "pos.go"},
		},
		"unknown_flag_value_looks_like_flag": {
			args:        []string{"-futureflag", "-other", "pos.go"},
			expectPosit: []string{"pos.go"},
		},
		"unknown_flag_at_end": {
			args:        []string{"pos.go", "-futureflag"},
			expectPosit: []string{"pos.go", "-futureflag"},
		},
		"unknown_flag_after_positional": {
			// Flags are not parsed past the first positional argument, mirroring
			// [flag.FlagSet.Parse] behavior.
			args:        []string{"pos.go", "-exportfd=3"},
			expectPosit: []string{"pos.go", "-exportfd=3"},
		},
		"double_dash_terminator": {
			args:        []string{"--", "-exportfd=3"},
			expectPosit: []string{"-exportfd=3"},
		},
		"known_flag_missing_value": {
			// Malformed known flags still error, same as [flag.FlagSet.Parse].
			args:        []string{"-foo"},
			expectError: "flag needs an argument: -foo",
		},
	} {
		t.Run(name, func(t *testing.T) {
			flagSet := flag.NewFlagSet("test", flag.ContinueOnError)
			foo := flagSet.String("foo", "", "")
			bar := flagSet.Bool("bar", false, "")

			posit, err := parseToolFlags(flagSet, tc.args)
			if tc.expectError != "" {
				require.EqualError(t, err, tc.expectError)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.expectPosit, posit)
			require.Equal(t, tc.expectFoo, *foo)
			require.Equal(t, tc.expectBar, *bar)
		})
	}
}
