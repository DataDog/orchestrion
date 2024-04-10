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
				Short: []string{"-short1"},
			},
			remove: []string{"-notfound"},
		},
		"found": {
			flags: CommandFlags{
				Long:  map[string]string{"-long1": "long1val", "-long2": "long2val"},
				Short: []string{"-short1", "-short2"},
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
		expected CommandFlags
	}{
		"short": {
			flags:    []string{"-short1", "-short2"},
			expected: CommandFlags{Short: []string{"-short1", "-short2"}},
		},
		"long": {
			flags:    []string{"-long1", "longval1", "-long2", "longval2"},
			expected: CommandFlags{Long: map[string]string{"-long1": "longval1", "-long2": "longval2"}},
		},
		"long-assigned": {
			flags:    []string{"-long1=longval1", "-long2=longval2"},
			expected: CommandFlags{Long: map[string]string{"-long1": "longval1", "-long2": "longval2"}},
		},
		"long-mixed": {
			flags:    []string{"-long1=longval1", "-long2", "longval2"},
			expected: CommandFlags{Long: map[string]string{"-long1": "longval1", "-long2": "longval2"}},
		},
		"combined": {
			flags: []string{"-short1", "-long1=longval1", "-short2", "-long2", "longval2"},
			expected: CommandFlags{
				Long:  map[string]string{"-long1": "longval1", "-long2": "longval2"},
				Short: []string{"-short1", "-short2"},
			},
		},
		"combined-and-skipped": {
			flags: []string{"skipped1", "-short1", "-long1=longval1", "skipped2", "-short2", "-long2", "longval2", "skipped3"},
			expected: CommandFlags{
				Long:  map[string]string{"-long1": "longval1", "-long2": "longval2"},
				Short: []string{"-short1", "-short2"},
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			flags := ParseCommandFlags(tc.flags)
			if len(tc.expected.Short) > 0 {
				require.True(t, reflect.DeepEqual(tc.expected.Short, flags.Short))
			}
			if len(tc.expected.Long) > 0 {
				require.True(t, reflect.DeepEqual(tc.expected.Long, flags.Long))
			}
		})
	}
}
