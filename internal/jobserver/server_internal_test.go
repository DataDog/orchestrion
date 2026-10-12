// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package jobserver

import (
	"context"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClose(t *testing.T) {
	// The server creates its temporary directory in the default directory for temporary files.
	tmp := t.TempDir()
	for _, name := range []string{"TMPDIR", "TMP", "TEMP"} {
		t.Setenv(name, tmp)
	}

	srv, err := New(context.Background(), &Options{NoListener: true})
	require.NoError(t, err)
	dirs, err := filepath.Glob(filepath.Join(tmp, "orchestrion.nbt-*"))
	require.NoError(t, err)
	require.Len(t, dirs, 1, "the server's temporary directory was not found")

	var hookRuns atomic.Int32
	srv.onShutdown(func(context.Context) error {
		time.Sleep(100 * time.Millisecond)
		hookRuns.Add(1)
		return nil
	})

	// Close returns once the shutdown hooks have run, and the temporary directory is removed...
	srv.Close()
	assert.EqualValues(t, 1, hookRuns.Load(), "Close returned before the shutdown hooks completed")
	assert.NoDirExists(t, dirs[0])

	// ... and the hooks never run again.
	srv.WaitForShutdown()
	assert.EqualValues(t, 1, hookRuns.Load(), "the shutdown hooks ran more than once")
}
