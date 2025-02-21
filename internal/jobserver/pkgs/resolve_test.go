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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/DataDog/orchestrion/internal/goflags"
	"github.com/DataDog/orchestrion/internal/jobserver"
	"github.com/DataDog/orchestrion/internal/jobserver/client"
	"github.com/DataDog/orchestrion/internal/jobserver/pkgs"
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
