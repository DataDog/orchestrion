// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package config

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoaderFiles(t *testing.T) {
	const aspectsYML = `meta:
  name: test
  description: test
aspects:
  - id: test
    join-point:
      directive: test:marker
    advice:
      - prepend-statements:
          template: _ = 0
`

	dir := t.TempDir()
	writeFile := func(name, content string) {
		t.Helper()
		path := filepath.Join(dir, filepath.FromSlash(name))
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	}
	writeFile("go.mod", "module example.com/files\n\ngo 1.25\n")
	writeFile(FilenameOrchestrionToolGo, "//go:build tools\n\npackage tools\n\nimport (\n\t_ \"example.com/files/instrumentation\"\n\t_ \"example.com/files/noconfig\"\n)\n")
	writeFile("instrumentation/instrumentation.go", "package instrumentation\n")
	writeFile("noconfig/noconfig.go", "package noconfig\n")
	writeFile("instrumentation/"+FilenameOrchestrionYML, aspectsYML)

	loader := NewLoader(nil, dir, false)
	cfg, err := loader.Load(context.Background())
	require.NoError(t, err)
	require.Len(t, cfg.Aspects(), 1)

	files := loader.Files()
	// Files that were read have a digest...
	assert.NotNil(t, files[filepath.Join(dir, FilenameOrchestrionToolGo)])
	assert.NotNil(t, files[filepath.Join(dir, "instrumentation", FilenameOrchestrionYML)])
	// ... and files that were looked for but do not exist are recorded without one.
	for _, absent := range []string{
		filepath.Join(dir, FilenameOrchestrionYML),
		filepath.Join(dir, "instrumentation", FilenameOrchestrionToolGo),
	} {
		digest, found := files[absent]
		assert.True(t, found, "absent file %q is not recorded", absent)
		assert.Nil(t, digest, "absent file %q has a digest", absent)
	}
	assert.True(t, files.Unchanged())

	// Changing the content of a file that was read is detected, even if its size is unchanged.
	writeFile("instrumentation/"+FilenameOrchestrionYML, aspectsYML[:len(aspectsYML)-2]+"1\n")
	assert.False(t, files.Unchanged())
	writeFile("instrumentation/"+FilenameOrchestrionYML, aspectsYML)
	assert.True(t, files.Unchanged())

	// Replacing the directory of files that were looked for but did not exist (here, a package without
	// any configuration file) is detected; on Windows, these files are then also reported as missing.
	noconfig := filepath.Join(dir, "noconfig")
	require.NoError(t, os.RemoveAll(noconfig))
	assert.False(t, files.Unchanged())
	require.NoError(t, os.WriteFile(noconfig, nil, 0o644))
	assert.False(t, files.Unchanged())
	require.NoError(t, os.Remove(noconfig))
	writeFile("noconfig/noconfig.go", "package noconfig\n")
	assert.True(t, files.Unchanged())

	// Creating a file that was looked for but did not exist is detected.
	writeFile(FilenameOrchestrionYML, aspectsYML)
	assert.False(t, files.Unchanged())
}
