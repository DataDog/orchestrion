// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package client

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/DataDog/orchestrion/internal/filelock"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWaitForURLFileAfterServerExit(t *testing.T) {
	// A job server process that exits with status 0, without ever writing the URL file (as happens when
	// it fails to start).
	cmd := exec.Command(os.Args[0], "-test.run=^$")
	require.NoError(t, cmd.Start())
	exitChan := make(chan error)
	go func() {
		defer close(exitChan)
		exitChan <- cmd.Wait()
	}()

	var logs bytes.Buffer
	ctx, cancel := context.WithTimeout(zerolog.New(&logs).Level(zerolog.TraceLevel).WithContext(context.Background()), 500*time.Millisecond)
	defer cancel()

	_, err := waitForURLFile(ctx, filepath.Join(t.TempDir(), urlFileName), cmd, exitChan)
	require.Error(t, err)

	// It keeps polling the URL file at its usual pace until the timeout, rather than in a busy loop.
	attempts := bytes.Count(logs.Bytes(), []byte("Job server still not ready"))
	t.Logf("The URL file was checked %d times in 500ms", attempts)
	assert.Less(t, attempts, 20)
}

func TestWaitForURLFileWrittenAfterServerExit(t *testing.T) {
	t.Setenv(EnvVarJobserverURL, "") // Set by waitForURLFile once connected
	t.Cleanup(func() {
		if client != nil {
			client.Close()
			client = nil
		}
	})

	// A job server started by another process, which only advertises its URL after the job server
	// process started by this one has exited with status 0.
	srv, err := server.NewServer(&server.Options{Host: "127.0.0.1", Port: server.RANDOM_PORT, NoLog: true, NoSigs: true})
	require.NoError(t, err)
	srv.Start()
	t.Cleanup(srv.Shutdown)
	require.True(t, srv.ReadyForConnections(5*time.Second))

	cmd := exec.Command(os.Args[0], "-test.run=^$")
	require.NoError(t, cmd.Start())
	exitChan := make(chan error)
	go func() {
		defer close(exitChan)
		exitChan <- cmd.Wait()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	urlFile := filepath.Join(t.TempDir(), urlFileName)
	const delay = 300 * time.Millisecond
	time.AfterFunc(delay, func() {
		file := filelock.MutexAt(urlFile)
		if err := file.Lock(ctx); err == nil {
			_, _ = file.Write([]byte(srv.ClientURL()))
			_ = file.Unlock(ctx)
		}
	})

	// It connects at the next check of the URL file once it is written.
	start := time.Now()
	c, err := waitForURLFile(ctx, urlFile, cmd, exitChan)
	require.NoError(t, err)
	require.NotNil(t, c)
	assert.Less(t, time.Since(start), delay+time.Second)
}
