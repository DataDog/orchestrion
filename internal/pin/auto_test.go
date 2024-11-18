// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package pin

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/DataDog/orchestrion/internal/injector/config"
	"github.com/DataDog/orchestrion/internal/version"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAutoPin(t *testing.T) {
	t.Run("simple", func(t *testing.T) {
		if cwd, err := os.Getwd(); err == nil {
			defer require.NoError(t, os.Chdir(cwd))
		}

		tmp := scaffold(t, make(map[string]string))
		require.NoError(t, os.Chdir(tmp))
		AutoPinOrchestrion()

		assert.NotEmpty(t, os.Getenv(envVarCheckedGoMod))

		assert.FileExists(t, filepath.Join(tmp, config.FilenameOrchestrionToolGo))
		assert.FileExists(t, filepath.Join(tmp, "go.sum"))

		data, err := parseGoMod(filepath.Join(tmp, "go.mod"))
		require.NoError(t, err)

		assert.Contains(t, data.Require, goModRequire{"github.com/DataDog/orchestrion", version.Tag})
	})

	t.Run("already-checked", func(t *testing.T) {
		tmp := scaffold(t, make(map[string]string))
		require.NoError(t, os.Chdir(tmp))

		require.NoError(t, os.Remove("go.mod"))

		t.Setenv(envVarCheckedGoMod, "true")

		AutoPinOrchestrion()

		assert.NoFileExists(t, filepath.Join(tmp, config.FilenameOrchestrionToolGo))
		assert.NoFileExists(t, filepath.Join(tmp, "go.sum"))
	})
}
