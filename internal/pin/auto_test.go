// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package pin

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/DataDog/orchestrion/internal/injector/config"
	"github.com/DataDog/orchestrion/internal/version"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAutoPin(t *testing.T) {
	ctx := context.Background()
	if d, ok := t.Deadline(); ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithDeadline(ctx, d)
		defer cancel()
	}

	t.Run("simple", func(t *testing.T) {
		tmp := scaffold(t, make(map[string]string))
		chdir(t, tmp)

		require.NoError(t, AutoPinOrchestrion(context.Background(), io.Discard, io.Discard))

		assert.NotEmpty(t, os.Getenv(envVarCheckedGoMod))

		assert.FileExists(t, filepath.Join(tmp, config.FilenameOrchestrionToolGo))
		assert.FileExists(t, filepath.Join(tmp, "go.sum"))

		data, err := parseGoMod(ctx, filepath.Join(tmp, "go.mod"))
		require.NoError(t, err)

		rawTag, _ := version.TagInfo()
		assert.Contains(t, data.Require, goModRequire{"github.com/DataDog/orchestrion", rawTag})
	})

	t.Run("already-checked", func(t *testing.T) {
		tmp := scaffold(t, make(map[string]string))
		chdir(t, tmp)

		require.NoError(t, os.Remove("go.mod"))

		t.Setenv(envVarCheckedGoMod, "true")

		require.NoError(t, AutoPinOrchestrion(ctx, io.Discard, io.Discard))

		assert.NoFileExists(t, filepath.Join(tmp, config.FilenameOrchestrionToolGo))
		assert.NoFileExists(t, filepath.Join(tmp, "go.sum"))
	})
}
