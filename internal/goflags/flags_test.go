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
		expected CommandFlags
	}{
		"short": {
			flags:    []string{"-short1", "-short2"},
			expected: CommandFlags{Short: map[string]struct{}{"-short1": {}, "-short2": {}}},
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
		"special": {
			flags: []string{"-gcflags", "-N -l -other", "-ldflags", "-extldflags '-lm -lstdc++ -static'"},
			expected: CommandFlags{
				Long: map[string]string{"-gcflags": "-N -l -other", "-ldflags": "-extldflags '-lm -lstdc++ -static'"},
			},
		},
		"combined": {
			flags: []string{"-short1", "-gcflags", "-N -l -other", "-ldflags", "-extldflags '-lm -lstdc++ -static'", "-long1=longval1", "-short2", "-long2", "longval2"},
			expected: CommandFlags{
				Long:  map[string]string{"-gcflags": "-N -l -other", "-ldflags": "-extldflags '-lm -lstdc++ -static'", "-long1": "longval1", "-long2": "longval2"},
				Short: map[string]struct{}{"-short1": {}, "-short2": {}},
			},
		},
		"combined-and-unknown": {
			flags: []string{"unknown1", "-short1", "-long1=longval1", "unknown2", "-short2", "-long2", "longval2", "unknown3"},
			expected: CommandFlags{
				Long:  map[string]string{"-long1": "longval1", "-long2": "longval2"},
				Short: map[string]struct{}{"-short1": {}, "-short2": {}},
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			shortFlags = tc.expected.Short
			longFlags = map[string]struct{}{}
			for flag := range tc.expected.Long {
				longFlags[flag] = struct{}{}
			}
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
