// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package proxy

import (
	gocontext "context"
	"os"
	"path/filepath"
	"testing"

	"github.com/DataDog/orchestrion/internal/injector/aspect/context"
	"github.com/DataDog/orchestrion/internal/jobserver"
	"github.com/DataDog/orchestrion/internal/jobserver/client"
	"github.com/stretchr/testify/require"
)

func TestParseCompile(t *testing.T) {
	work := t.TempDir()
	js, err := jobserver.New(gocontext.Background(), nil)
	require.NoError(t, err)
	defer js.Shutdown()
	t.Setenv(client.EnvVarJobserverURL, js.ClientURL())

	importCfgFile := filepath.Join(work, "b002", "importcfg")
	require.NoError(t, os.MkdirAll(filepath.Dir(importCfgFile), 0o755))
	require.NoError(t, os.WriteFile(importCfgFile, nil, 0o644))

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
			input:   []string{"/path/compile", "-o", work + "/b002/a.out", "-p", "mypackage", "-lang=go1.42", "-buildid", "123456", "-importcfg", importCfgFile, "/source/dir/main.go", "/source/dir/file1.go"},
			goFiles: []string{"/source/dir/main.go", "/source/dir/file1.go"},
			flags: compileFlagSet{
				BuildID:   "123456",
				Package:   "mypackage",
				ImportCfg: importCfgFile,
				Output:    work + "/b002/a.out",
				Lang:      "go1.42",
			},
		},
		"buildid": {
			input:   []string{"/path/compile", "-o", work + "/b019/_pkg_.a", "-trimpath", work + "=>", "-p", "internal/profilerecord", "-lang=go1.23", "-std", "-complete", "-buildid", "58eel3bXIltdLxQE0aV1/58eel3bXIltdLxQE0aV1", "-goversion", "go1.23.4", "-c=4", "-shared", "-nolocalimports", "-importcfg", importCfgFile, "-pack", "/go/src/internal/profilerecord/profilerecord.go"},
			goFiles: []string{"/go/src/internal/profilerecord/profilerecord.go"},
			flags: compileFlagSet{
				Package:   "internal/profilerecord",
				ImportCfg: importCfgFile,
				Output:    work + "/b019/_pkg_.a",
				Lang:      "go1.23",
				BuildID:   "58eel3bXIltdLxQE0aV1/58eel3bXIltdLxQE0aV1",
			},
		},
		"nats.go": {
			input:   []string{"/path/compile", "-o", work + "/b002/a.out", "-p", "github.com/nats-io/nats.go", "-complete", "/path/to/source/file.go"},
			goFiles: []string{"/path/to/source/file.go"},
			flags: compileFlagSet{
				Package: "github.com/nats-io/nats.go",
				Output:  work + "/b002/a.out",
			},
		},
	} {
		if tc.goFiles == nil {
			// Simplify comparisons, as goFiles always returns non-nil
			tc.goFiles = make([]string, 0)
		}

		t.Run(name, func(t *testing.T) {
			cmd, err := parseCompileCommand(gocontext.Background(), "github.com/DataDog/orchestrion.test/"+name, tc.input)
			require.NoError(t, err)
			require.Equal(t, CommandTypeCompile, cmd.Type())
			require.Equal(t, tc.flags, cmd.Flags)
			require.Equal(t, tc.goFiles, cmd.GoFiles())
		})
	}
}

func TestSetLang(t *testing.T) {
	work := t.TempDir()

	t.Run("-lang go1.13", func(t *testing.T) {
		cmd, err := parseCompileCommand(gocontext.Background(), "github.com/DataDog/orchestrion.test", []string{
			"/path/to/compile",
			"-o", work + "/b002/a.out",
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
		cmd, err := parseCompileCommand(gocontext.Background(), "github.com/DataDog/orchestrion.test", []string{
			"/path/to/compile",
			"-o", work + "/b002/a.out",
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
		cmd, err := parseCompileCommand(gocontext.Background(), "github.com/DataDog/orchestrion.test", []string{
			"/path/to/compile",
			"-o", work + "/b002/a.out",
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
			"-o", work + "/b002/a.out",
			"source/file.go",
		}

		cmd, err := parseCompileCommand(gocontext.Background(), "github.com/DataDog/orchestrion.test", args)
		require.NoError(t, err)

		require.NoError(t, cmd.SetLang(context.GoLangVersion{}))
		require.Equal(t, args, cmd.Args())

		require.NoError(t, cmd.SetLang(context.MustParseGoLangVersion("go1.18")))
		require.Equal(t, args, cmd.Args())
	})
}
