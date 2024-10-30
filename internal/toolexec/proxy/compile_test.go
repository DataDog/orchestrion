// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package proxy

import (
	"testing"

	"github.com/DataDog/orchestrion/internal/injector/aspect/context"
	"github.com/stretchr/testify/require"
)

func TestParseCompile(t *testing.T) {
	for name, tc := range map[string]struct {
		input   []string
		goFiles []string
		flags   compileFlagSet
	}{
		"version_print": {
			input: []string{"/path/compile", "-V=full"},
			flags: compileFlagSet{
				ShowVersion: true,
			},
		},
		"compile": {
			input:   []string{"/path/compile", "-o", "/buildDir/b002/a.out", "-p", "mypackage", "-lang=go1.42", "-importcfg", "/buildDir/b002/importcfg", "/source/dir/main.go", "/source/dir/file1.go"},
			goFiles: []string{"/source/dir/main.go", "/source/dir/file1.go"},
			flags: compileFlagSet{
				Package:   "mypackage",
				ImportCfg: "/buildDir/b002/importcfg",
				Output:    "/buildDir/b002/a.out",
				Lang:      "go1.42",
			},
		},
		"nats.go": {
			input:   []string{"/path/compile", "-o", "/buildDir/b002/a.out", "-p", "github.com/nats-io/nats.go", "-complete", "/path/to/source/file.go"},
			goFiles: []string{"/path/to/source/file.go"},
			flags: compileFlagSet{
				Package: "github.com/nats-io/nats.go",
				Output:  "/buildDir/b002/a.out",
			},
		},
	} {
		if tc.goFiles == nil {
			// Simplify comparisons, as goFiles always returns non-nil
			tc.goFiles = make([]string, 0)
		}

		if name != "nats.go" {
			continue
		}
		t.Run(name, func(t *testing.T) {
			cmd, err := parseCompileCommand(tc.input)
			require.NoError(t, err)
			require.Equal(t, CommandTypeCompile, cmd.Type())
			require.Equal(t, tc.flags, cmd.Flags)
			require.EqualValues(t, tc.goFiles, cmd.GoFiles())
		})
	}
}

func TestSetLang(t *testing.T) {
	t.Run("-lang go1.13", func(t *testing.T) {
		cmd, err := parseCompileCommand([]string{
			"/path/to/compile",
			"-o", "/buildDir/b002/a.out",
			"-lang", "go1.13",
			"source/file.go",
		})
		require.NoError(t, err)
		require.Equal(t, "go1.13", cmd.Args()[4])

		require.NoError(t, cmd.SetLang(context.GoLangVersion{}))
		require.Equal(t, "go1.13", cmd.Args()[4])

		require.NoError(t, cmd.SetLang(context.MustParseGoLangVersion("go1.18")))
		require.Equal(t, "go1.18", cmd.Args()[4])
	})

	t.Run("-lang go1.23", func(t *testing.T) {
		cmd, err := parseCompileCommand([]string{
			"/path/to/compile",
			"-o", "/buildDir/b002/a.out",
			"-lang", "go1.23",
			"source/file.go",
		})
		require.NoError(t, err)
		require.Equal(t, "go1.23", cmd.Args()[4])

		require.NoError(t, cmd.SetLang(context.GoLangVersion{}))
		require.Equal(t, "go1.23", cmd.Args()[4])

		require.NoError(t, cmd.SetLang(context.MustParseGoLangVersion("go1.18")))
		require.Equal(t, "go1.23", cmd.Args()[4])
	})

	t.Run("-lang=go1.13", func(t *testing.T) {
		cmd, err := parseCompileCommand([]string{
			"/path/to/compile",
			"-o", "/buildDir/b002/a.out",
			"-lang=go1.13",
			"source/file.go",
		})
		require.NoError(t, err)

		require.NoError(t, cmd.SetLang(context.GoLangVersion{}))
		require.Equal(t, "-lang=go1.13", cmd.Args()[3])

		require.NoError(t, cmd.SetLang(context.MustParseGoLangVersion("go1.18")))
		require.Equal(t, "-lang=go1.18", cmd.Args()[3])
	})

	t.Run("no -lang flag", func(t *testing.T) {
		args := []string{
			"/path/to/compile",
			"-o", "/buildDir/b002/a.out",
			"source/file.go",
		}

		cmd, err := parseCompileCommand(args)
		require.NoError(t, err)

		require.NoError(t, cmd.SetLang(context.GoLangVersion{}))
		require.Equal(t, args, cmd.Args())

		require.NoError(t, cmd.SetLang(context.MustParseGoLangVersion("go1.18")))
		require.Equal(t, args, cmd.Args())
	})
}
