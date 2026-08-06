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
	"strings"
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
		"coverprofile-implies-cover": {
			// `-coverprofile` is not a build flag (it must not be forwarded to child builds), but it implies
			// coverage instrumentation is enabled, so child builds must apply it to the same packages.
			flags: []string{"test", "-coverprofile=coverage.out", "./..."},
			expected: CommandFlags{
				Long: map[string]string{
					"-coverpkg": "github.com/DataDog/orchestrion/internal/goflags,github.com/DataDog/orchestrion/internal/goflags/quoted",
				},
				Short:   map[string]struct{}{"-cover": {}},
				Unknown: []string{"-coverprofile=coverage.out"},
			},
			useStdFlags: true,
		},
		"test-coverprofile-implies-cover": {
			// The Go CLI accepts test flags with a `test.` prefix, too.
			flags: []string{"test", "-test.coverprofile", "coverage.out", "."},
			expected: CommandFlags{
				Long:    map[string]string{"-coverpkg": "github.com/DataDog/orchestrion/internal/goflags"},
				Short:   map[string]struct{}{"-cover": {}},
				Unknown: []string{"-test.coverprofile", "coverage.out"},
			},
			useStdFlags: true,
		},
		"valueless-unknown-flag-keeps-positional": {
			// `-v` accepts no value, so `./quoted` is a positional argument (and hence, the only package
			// coverage must be applied to).
			flags: []string{"test", "-coverprofile=coverage.out", "-v", "./quoted"},
			expected: CommandFlags{
				Long:    map[string]string{"-coverpkg": "github.com/DataDog/orchestrion/internal/goflags/quoted"},
				Short:   map[string]struct{}{"-cover": {}},
				Unknown: []string{"-coverprofile=coverage.out", "-v"},
			},
			useStdFlags: true,
		},
		"value-accepting-unknown-flag-consumes-value": {
			// `-run` accepts a value, so `TestFoo` is not a positional argument.
			flags: []string{"test", "-cover", "-run", "TestFoo", "./quoted"},
			expected: CommandFlags{
				Long:    map[string]string{"-coverpkg": "github.com/DataDog/orchestrion/internal/goflags/quoted"},
				Short:   map[string]struct{}{"-cover": {}},
				Unknown: []string{"-run", "TestFoo"},
			},
			useStdFlags: true,
		},
		"package-patterns-are-a-contiguous-run": {
			// The Go CLI stops accepting package patterns once a flag follows them, so `./quoted` is an
			// argument destined to the test binary here, and not a package pattern.
			flags: []string{"test", "-coverprofile=coverage.out", ".", "-v", "./quoted"},
			expected: CommandFlags{
				Long:    map[string]string{"-coverpkg": "github.com/DataDog/orchestrion/internal/goflags"},
				Short:   map[string]struct{}{"-cover": {}},
				Unknown: []string{"-coverprofile=coverage.out", "-v"},
			},
			useStdFlags: true,
		},
		"coverprofile-is-ignored-outside-go-test": {
			// The Go CLI silently ignores flags from $GOFLAGS that the command at hand does not accept, so
			// `go build` does not enable coverage here; and neither must child builds.
			flags:   []string{"build", "./quoted"},
			goflags: "-coverprofile=coverage.out",
			expected: CommandFlags{
				Unknown: []string{"-coverprofile=coverage.out"},
			},
			useStdFlags: true,
		},
		"go-test-ignores-patterns-after-terminator": {
			// `go test` never accepts package patterns after the "--" marker; everything that follows is
			// destined to the test binary, and coverage hence applies to the working directory's package.
			flags: []string{"test", "-coverprofile=coverage.out", "--", "./quoted"},
			expected: CommandFlags{
				Long:    map[string]string{"-coverpkg": "github.com/DataDog/orchestrion/internal/goflags"},
				Short:   map[string]struct{}{"-cover": {}},
				Unknown: []string{"-coverprofile=coverage.out"},
			},
			useStdFlags: true,
		},
		"args-terminates-parsing": {
			// Everything after `-args` is destined to the test binary.
			flags: []string{"test", "-cover", "-args", "./quoted", "-some-flag"},
			expected: CommandFlags{
				// No package pattern was provided, so coverage applies to the package in the working directory.
				Long:  map[string]string{"-coverpkg": "github.com/DataDog/orchestrion/internal/goflags"},
				Short: map[string]struct{}{"-cover": {}},
			},
			useStdFlags: true,
		},
		"bare-buildvcs-accepts-no-value": {
			// `-buildvcs` accepts an optional value, so `./quoted` is a package pattern, and not the flag's
			// value.
			flags: []string{"test", "-cover", "-buildvcs", "./quoted"},
			expected: CommandFlags{
				Long:  map[string]string{"-coverpkg": "github.com/DataDog/orchestrion/internal/goflags/quoted"},
				Short: map[string]struct{}{"-cover": {}, "-buildvcs": {}},
			},
			useStdFlags: true,
		},
		"assigned-buildvcs-keeps-its-value": {
			flags: []string{"test", "-cover", "-buildvcs=false", "./quoted"},
			expected: CommandFlags{
				Long: map[string]string{
					"-buildvcs": "false",
					"-coverpkg": "github.com/DataDog/orchestrion/internal/goflags/quoted",
				},
				Short: map[string]struct{}{"-cover": {}},
			},
			useStdFlags: true,
		},
		"trailing-flag-without-value": {
			// A value-accepting flag with no value is invalid; the Go CLI reports the error in its canonical
			// way, and we must not panic trying to read the missing value.
			flags: []string{"test", "-cover", "./quoted", "-tags"},
			expected: CommandFlags{
				Long:    map[string]string{"-coverpkg": "github.com/DataDog/orchestrion/internal/goflags/quoted"},
				Short:   map[string]struct{}{"-cover": {}},
				Unknown: []string{"-tags"},
			},
			useStdFlags: true,
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

			if tc.expected.Unknown != nil {
				assert.Equal(t, tc.expected.Unknown, flags.Unknown)
			}

			// Flags that are not build flags must never be forwarded to child commands.
			for _, flag := range flags.Slice() {
				name, _, _ := strings.Cut(flag, "=")
				assert.False(t, impliesCover(name), "flag %q must not be forwarded to child commands", flag)
				assert.False(t, isValueless(name), "flag %q must not be forwarded to child commands", flag)
			}
		})
	}
}

func restore(short map[string]struct{}, long map[string]struct{}) {
	shortFlags = short
	longFlags = long
}
