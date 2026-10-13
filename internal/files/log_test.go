// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package files_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/DataDog/orchestrion/internal/files"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAppendLog(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "logs")
	require.NoError(t, os.Mkdir(dir, 0o755))
	path := filepath.Join(dir, "test.log")
	require.NoError(t, os.WriteFile(path, []byte("existing\n"), 0o644))

	file, err := files.AppendLog(path)
	require.NoError(t, err)
	defer file.Close()
	_, err = file.WriteString("written\n")
	require.NoError(t, err)
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "existing\nwritten\n", string(content))

	// The file, and the directory containing it, can be removed while the file is open (which
	// os.OpenFile does not allow on Windows); and writing to it is still possible afterwards.
	require.NoError(t, os.RemoveAll(dir))
	_, err = file.WriteString("written after removal\n")
	require.NoError(t, err)

	// The file is created if it does not exist.
	created, err := files.AppendLog(filepath.Join(t.TempDir(), "new.log"))
	require.NoError(t, err)
	require.NoError(t, created.Close())
}
