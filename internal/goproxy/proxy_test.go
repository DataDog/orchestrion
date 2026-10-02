// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package goproxy_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/DataDog/orchestrion/internal/goproxy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunRemovesJobServerFiles(t *testing.T) {
	// The job server writes its temporary files in the default directory for temporary files.
	tmp := t.TempDir()
	for _, name := range []string{"TMPDIR", "TMP", "TEMP"} {
		t.Setenv(name, tmp)
	}

	// A module without any Go file: the go command fails right away, but only after the job server was
	// started for it.
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/empty\n\ngo 1.25\n"), 0o644))
	t.Chdir(dir)

	err := goproxy.Run(context.Background(), []string{"build", "."}, goproxy.WithToolexec("unused-toolexec"))
	require.Error(t, err)

	// Once Run returns, the job server has completely shut down, and removed its temporary files.
	leftovers, err := filepath.Glob(filepath.Join(tmp, "orchestrion.nbt-*"))
	require.NoError(t, err)
	assert.Empty(t, leftovers, "the job server's temporary files were not removed")
}
