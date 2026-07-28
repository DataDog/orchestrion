// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package pkgs_test

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/DataDog/orchestrion/internal/goflags"
	"github.com/DataDog/orchestrion/internal/jobserver"
	"github.com/DataDog/orchestrion/internal/jobserver/client"
	"github.com/DataDog/orchestrion/internal/jobserver/pkgs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test(t *testing.T) {
	// Force the goflags so we don't get tainted by the `go test` flags!
	wd, err := os.Getwd()
	require.NoError(t, err)
	goflags.SetFlags(context.Background(), wd, []string{"test"})

	t.Run("Cache", func(t *testing.T) {
		server, err := jobserver.New(context.Background(), nil)
		require.NoError(t, err)
		defer server.Shutdown()

		conn, err := server.Connect()
		require.NoError(t, err)
		defer conn.Close()

		env := os.Environ()

		// First request is expected to always be a cache miss
		resp, err := client.Request(
			context.Background(),
			conn,
			&pkgs.ResolveRequest{
				Pattern: "net/http",
				Env:     env,
			},
		)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(resp), 2)
		assert.EqualValues(t, 1, server.CacheStats.Count())
		assert.EqualValues(t, 0, server.CacheStats.Hits())

		// Second request is equivalent, and should result in a cache hit. The order
		// of entries in `env` is also shuffled, which should have no impact on the
		// cache hitting or missing.
		rand.Shuffle(len(env), func(i, j int) { env[i], env[j] = env[j], env[i] })
		resp, err = client.Request(
			context.Background(),
			conn,
			&pkgs.ResolveRequest{
				Pattern: "net/http",
				Env:     env, // This was shuffled, so it's not the same as before
			},
		)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(resp), 2)
		assert.EqualValues(t, 2, server.CacheStats.Count())
		assert.EqualValues(t, 1, server.CacheStats.Hits())

		// Third request is different, should result in a cache miss again
		resp, err = client.Request(
			context.Background(),
			conn,
			&pkgs.ResolveRequest{
				Pattern: "os", // Not the same package as before...
				Env:     env,
			},
		)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(resp), 3)
		assert.EqualValues(t, 3, server.CacheStats.Count())
		assert.EqualValues(t, 1, server.CacheStats.Hits())
	})

	t.Run("TestVariants", func(t *testing.T) {
		server, err := jobserver.New(context.Background(), nil)
		require.NoError(t, err)
		defer server.Shutdown()

		conn, err := server.Connect()
		require.NoError(t, err)
		defer conn.Close()

		dir := t.TempDir()
		writeFile := func(name, contents string) {
			t.Helper()
			path := filepath.Join(dir, filepath.FromSlash(name))
			require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
			require.NoError(t, os.WriteFile(path, []byte(contents), 0o644))
		}
		writeFile("go.mod", "module example.com/testvariants\n\ngo 1.25\n")
		writeFile("subject/subject.go", "package subject\n\nconst Value = 42\n")
		writeFile("subject/subject_test.go", "package subject\n\nimport \"testing\"\n\nfunc TestValue(t *testing.T) { if Value != 42 { t.Fail() } }\n")
		writeFile("middle/middle.go", "package middle\n\nimport \"example.com/testvariants/subject\"\n\nconst Value = subject.Value\n")
		writeFile("root/root.go", "package root\n\nimport \"example.com/testvariants/middle\"\n\nconst Value = middle.Value\n")
		writeFile("unrelated/unrelated.go", "package unrelated\n\nconst Value = 42\n")

		resp, err := client.Request(
			context.Background(),
			conn,
			&pkgs.ResolveRequest{
				Dir:            dir,
				Env:            os.Environ(),
				Pattern:        "example.com/testvariants/root",
				TestVariantFor: "example.com/testvariants/subject",
			},
		)
		require.NoError(t, err)
		require.Contains(t, resp, "example.com/testvariants/root")
		require.Contains(t, resp, "example.com/testvariants/middle")
		assert.NotEmpty(t, resp["example.com/testvariants/root"].ExportFile)
		assert.Equal(t, "example.com/testvariants/subject", resp["example.com/testvariants/root"].ForTest)
		assert.NotEmpty(t, resp["example.com/testvariants/middle"].ExportFile)
		assert.Equal(t, "example.com/testvariants/subject", resp["example.com/testvariants/middle"].ForTest)
		assert.NotContains(t, resp, "example.com/testvariants/subject")

		resp, err = client.Request(
			context.Background(),
			conn,
			&pkgs.ResolveRequest{
				Dir:            dir,
				Env:            os.Environ(),
				Pattern:        "example.com/testvariants/unrelated",
				TestVariantFor: "example.com/testvariants/subject",
			},
		)
		require.NoError(t, err)
		assert.NotEmpty(t, resp["example.com/testvariants/unrelated"].ExportFile)
		assert.Empty(t, resp["example.com/testvariants/unrelated"].ForTest)

		// An unrelated dependency does not need to load or validate the test target.
		resp, err = client.Request(
			context.Background(),
			conn,
			&pkgs.ResolveRequest{
				Dir:            dir,
				Env:            os.Environ(),
				Pattern:        "example.com/testvariants/unrelated",
				TestVariantFor: "example.com/testvariants/does-not-exist",
			},
		)
		require.NoError(t, err)
		assert.NotEmpty(t, resp["example.com/testvariants/unrelated"].ExportFile)
		assert.Empty(t, resp["example.com/testvariants/unrelated"].ForTest)

		writeFile("externalsubject/subject.go", "package externalsubject\n\nconst Value = 42\n")
		writeFile("externalsubject/subject_test.go", "package externalsubject_test\n\nimport (\"testing\"; \"example.com/testvariants/externalsubject\")\n\nfunc TestValue(t *testing.T) { if externalsubject.Value != 42 { t.Fail() } }\n")
		writeFile("externalroot/root.go", "package externalroot\n\nimport \"example.com/testvariants/externalsubject\"\n\nconst Value = externalsubject.Value\n")
		resp, err = client.Request(
			context.Background(),
			conn,
			&pkgs.ResolveRequest{
				Dir:            dir,
				Env:            os.Environ(),
				Pattern:        "example.com/testvariants/externalroot",
				TestVariantFor: "example.com/testvariants/externalsubject",
			},
		)
		require.NoError(t, err)
		assert.NotEmpty(t, resp["example.com/testvariants/externalroot"].ExportFile)
		assert.Empty(t, resp["example.com/testvariants/externalroot"].ForTest)
		assert.NotContains(t, resp, "example.com/testvariants/externalsubject")
	})

	t.Run("Error", func(t *testing.T) {
		server, err := jobserver.New(context.Background(), nil)
		require.NoError(t, err)
		defer server.Shutdown()

		conn, err := server.Connect()
		require.NoError(t, err)
		defer conn.Close()

		resp, err := client.Request(
			context.Background(),
			conn,
			&pkgs.ResolveRequest{Pattern: "definitely.not/a@valid\x01package"},
		)
		assert.Nil(t, resp)
		assert.EqualValues(t, 0, server.CacheStats.Hits())
		require.Error(t, err)
	})
}

func init() {
	if len(os.Args) <= 2 || os.Args[1] != "toolexec" {
		return
	}

	// We're invoked with `toolexec` so pretend we're a toolexec proxy...
	cmd := exec.Command(os.Args[2], os.Args[3:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		if err, ok := err.(*exec.ExitError); ok {
			os.Exit(err.ExitCode())
			return
		}
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(0)
}
