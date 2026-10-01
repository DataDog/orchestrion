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

func TestLogFiles(t *testing.T) {
	for name, tc := range map[string]struct {
		open     func(string) (*os.File, error)
		expected string
	}{
		"CreateLog": {open: files.CreateLog, expected: "written\n"},
		"AppendLog": {open: files.AppendLog, expected: "existing\nwritten\n"},
	} {
		t.Run(name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "logs")
			require.NoError(t, os.Mkdir(dir, 0o755))
			path := filepath.Join(dir, "test.log")
			require.NoError(t, os.WriteFile(path, []byte("existing\n"), 0o644))

			file, err := tc.open(path)
			require.NoError(t, err)
			defer file.Close()

			_, err = file.WriteString("written\n")
			require.NoError(t, err)
			content, err := os.ReadFile(path)
			require.NoError(t, err)
			assert.Equal(t, tc.expected, string(content))

			// The file, and the directory containing it, can be removed while the file is open (which
			// os.OpenFile does not allow on Windows); and writing to it is still possible afterwards.
			require.NoError(t, os.RemoveAll(dir))
			assert.NoDirExists(t, dir)
			_, err = file.WriteString("written after removal\n")
			require.NoError(t, err)
		})
	}

	t.Run("missing", func(t *testing.T) {
		for _, open := range []func(string) (*os.File, error){files.CreateLog, files.AppendLog} {
			path := filepath.Join(t.TempDir(), "test.log")
			file, err := open(path)
			require.NoError(t, err)
			require.NoError(t, file.Close())
			assert.FileExists(t, path)
		}
	})
}
