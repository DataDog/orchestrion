// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package filelock_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/DataDog/orchestrion/internal/filelock"
	"github.com/stretchr/testify/require"
)

func Test(t *testing.T) {
	t.Run("RLock first", func(t *testing.T) {
		tmp := t.TempDir()
		lockfile := filepath.Join(tmp, "file.lock")

		mu := filelock.MutexAt(lockfile)
		require.NoError(t, mu.RLock(), "failed to acquire read lock")
		assertExists(t, lockfile)
		require.NoError(t, mu.Lock(), "failed to upgrade to write lock")
		require.NoError(t, mu.RLock(), "failed to downgrade to read lock")
		require.NoError(t, mu.Unlock(), "failed to unlock")
	})

	t.Run("Lock first", func(t *testing.T) {
		tmp := t.TempDir()
		lockfile := filepath.Join(tmp, "file.lock")

		mu := filelock.MutexAt(lockfile)
		require.NoError(t, mu.Lock(), "failed to acquire write lock")
		assertExists(t, lockfile)
		require.NoError(t, mu.RLock(), "failed to downgrade to read lock")
		require.NoError(t, mu.Lock(), "failed to upgrade to write lock")
		require.NoError(t, mu.Unlock(), "failed to unlock")
	})
}

func assertExists(t *testing.T, path string) {
	stat, err := os.Stat(path)
	require.NoError(t, err)
	require.NotNil(t, stat)
	require.False(t, stat.IsDir())
}
