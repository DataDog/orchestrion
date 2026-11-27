// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package goenv

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGOMOD(t *testing.T) {
	t.Run("without GOMOD environment variable", func(t *testing.T) {
		gomod, err := GOMOD("")
		require.NoError(t, err)
		require.NotEmpty(t, gomod)
	})

	t.Run("no GOMOD can be found", func(t *testing.T) {
		val, err := GOMOD(t.TempDir())
		require.Empty(t, val)
		require.ErrorIs(t, err, ErrNoGoMod)
	})
}

func TestGOWORK(t *testing.T) {
	dir := t.TempDir()
	workfile := filepath.Join(dir, "go.work")
	require.NoError(t, os.WriteFile(workfile, []byte("go 1.26.0\n"), 0o644))

	t.Run("workspace mode", func(t *testing.T) {
		t.Setenv("GOWORK", "auto")
		val, err := GOWORK(dir)
		require.NoError(t, err)
		require.Equal(t, workfile, val)
	})

	t.Run("GOWORK=off", func(t *testing.T) {
		// `go env GOWORK` prints "off" here, which must not be mistaken for a go.work file.
		t.Setenv("GOWORK", "off")
		val, err := GOWORK(dir)
		require.NoError(t, err)
		require.Empty(t, val)
	})
}
