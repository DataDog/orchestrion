// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package pkgs_test

import (
	"context"
	"math/rand"
	"os"
	"testing"

	"github.com/DataDog/orchestrion/internal/jobserver"
	"github.com/DataDog/orchestrion/internal/jobserver/client"
	"github.com/DataDog/orchestrion/internal/jobserver/pkgs"
	"github.com/stretchr/testify/require"
)

func TestCache(t *testing.T) {
	server, err := jobserver.New(nil)
	require.NoError(t, err)
	defer server.Shutdown()

	conn, err := server.Connect()
	require.NoError(t, err)
	defer conn.Close()

	env := os.Environ()

	// First request is expected to always be a cache miss
	resp, err := client.Request[*pkgs.ResolveRequest, pkgs.ResolveResponse](
		context.Background(),
		conn,
		&pkgs.ResolveRequest{
			Pattern: "net/http",
			Env:     env,
		},
	)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(resp), 2)
	require.EqualValues(t, 1, server.CacheStats.Count())
	require.EqualValues(t, 0, server.CacheStats.Hits())

	// Second request is equivalent, and should result in a cache hit. The order
	// of entries in `env` is also shuffled, which should have no impact on the
	// cache hitting or missing.
	rand.Shuffle(len(env), func(i, j int) { env[i], env[j] = env[j], env[i] })
	resp, err = client.Request[*pkgs.ResolveRequest, pkgs.ResolveResponse](
		context.Background(),
		conn,
		&pkgs.ResolveRequest{
			Pattern: "net/http",
			Env:     env, // This was shuffled, so it's not the same as before
		},
	)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(resp), 2)
	require.EqualValues(t, 2, server.CacheStats.Count())
	require.EqualValues(t, 1, server.CacheStats.Hits())

	// Third request is different, should result in a cache miss again
	resp, err = client.Request[*pkgs.ResolveRequest, pkgs.ResolveResponse](
		context.Background(),
		conn,
		&pkgs.ResolveRequest{
			Pattern: "os", // Not the same package as before...
			Env:     env,
		},
	)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(resp), 3)
	require.EqualValues(t, 3, server.CacheStats.Count())
	require.EqualValues(t, 1, server.CacheStats.Hits())
}

func TestError(t *testing.T) {
	server, err := jobserver.New(nil)
	require.NoError(t, err)
	defer server.Shutdown()

	conn, err := server.Connect()
	require.NoError(t, err)
	defer conn.Close()

	resp, err := client.Request[*pkgs.ResolveRequest, pkgs.ResolveResponse](
		context.Background(),
		conn,
		&pkgs.ResolveRequest{BuildFlags: []string{"--definitely-not-a-valid-build-flag"}, Pattern: "runtime"},
	)
	require.Error(t, err)
	require.Nil(t, resp)
}
