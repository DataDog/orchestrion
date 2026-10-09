// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package client

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFromEnvironmentReusesAdvertisedServer(t *testing.T) {
	t.Setenv(EnvVarJobserverURL, "")
	t.Cleanup(func() {
		if client != nil {
			client.Close()
			client = nil
		}
	})

	srv, err := server.NewServer(&server.Options{Host: "127.0.0.1", Port: server.RANDOM_PORT, NoLog: true, NoSigs: true})
	require.NoError(t, err)
	srv.Start()
	t.Cleanup(srv.Shutdown)
	require.True(t, srv.ReadyForConnections(5*time.Second))

	workDir := t.TempDir()
	urlFile := filepath.Join(workDir, urlFileName)
	require.NoError(t, os.WriteFile(urlFile, []byte(srv.ClientURL()), 0o644))

	c, err := FromEnvironment(context.Background(), workDir)
	require.NoError(t, err)
	require.NotNil(t, c)

	// No server process was spawned (doing so creates its log files next to the URL file).
	assert.NoFileExists(t, urlFile+".stderr.log")
	assert.Equal(t, srv.ClientURL(), os.Getenv(EnvVarJobserverURL))
}

func TestTryExistingServer(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()

	missing := filepath.Join(dir, "missing")
	_, _, ok := tryExistingServer(ctx, missing)
	assert.False(t, ok)
	assert.NoFileExists(t, missing, "the URL file must not be created")

	empty := filepath.Join(dir, "empty")
	require.NoError(t, os.WriteFile(empty, nil, 0o644))
	_, _, ok = tryExistingServer(ctx, empty)
	assert.False(t, ok)

	// A URL file left behind by a server that is no longer running.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := listener.Addr().String()
	require.NoError(t, listener.Close())
	stale := filepath.Join(dir, "stale")
	require.NoError(t, os.WriteFile(stale, []byte("nats://"+addr), 0o644))

	start := time.Now()
	_, _, ok = tryExistingServer(ctx, stale)
	assert.False(t, ok)
	assert.Less(t, time.Since(start), 5*time.Second, "a single connection attempt is made")
}
