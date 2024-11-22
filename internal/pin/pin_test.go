// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package pin

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"text/template"

	"github.com/DataDog/orchestrion/internal/injector/config"
	"github.com/DataDog/orchestrion/internal/version"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPin(t *testing.T) {
	t.Run("simple", func(t *testing.T) {
		tmp := scaffold(t, make(map[string]string))
		require.NoError(t, os.Chdir(tmp))

		require.NoError(t, PinOrchestrion(Options{}))

		assert.FileExists(t, filepath.Join(tmp, config.FilenameOrchestrionToolGo))
		assert.FileExists(t, filepath.Join(tmp, "go.sum"))

		data, err := parseGoMod(filepath.Join(tmp, "go.mod"))
		require.NoError(t, err)

		assert.Contains(t, data.Require, goModRequire{"github.com/DataDog/orchestrion", version.Tag})

		content, err := os.ReadFile(filepath.Join(tmp, config.FilenameOrchestrionToolGo))
		require.NoError(t, err)

		assert.Contains(t, string(content), "//go:generate")
	})

	t.Run("another-version", func(t *testing.T) {
		tmp := scaffold(t, map[string]string{"github.com/DataDog/orchestrion": "v0.9.3"})
		require.NoError(t, os.Chdir(tmp))

		require.NoError(t, PinOrchestrion(Options{}))

		assert.FileExists(t, filepath.Join(tmp, config.FilenameOrchestrionToolGo))
		assert.FileExists(t, filepath.Join(tmp, "go.sum"))

		data, err := parseGoMod(filepath.Join(tmp, "go.mod"))
		require.NoError(t, err)

		assert.Contains(t, data.Require, goModRequire{"github.com/DataDog/orchestrion", "v0.9.3"})
	})

	t.Run("no-generate", func(t *testing.T) {
		tmp := scaffold(t, make(map[string]string))
		require.NoError(t, os.Chdir(tmp))

		require.NoError(t, PinOrchestrion(Options{NoGenerate: true}))

		content, err := os.ReadFile(filepath.Join(tmp, config.FilenameOrchestrionToolGo))
		require.NoError(t, err)

		assert.NotContains(t, string(content), "//go:generate")
	})

	t.Run("prune", func(t *testing.T) {
		tmp := scaffold(t, map[string]string{"github.com/digitalocean/sample-golang": "v0.0.0-20240904143939-1e058723dcf4"})
		require.NoError(t, os.Chdir(tmp))

		require.NoError(t, PinOrchestrion(Options{NoGenerate: true}))

		data, err := parseGoMod(filepath.Join(tmp, "go.mod"))
		require.NoError(t, err)

		assert.NotContains(t, data.Require, goModRequire{"github.com/digitalocean/sample-golang", "v0.0.0-20240904143939-1e058723dcf4"})
	})

	t.Run("prune-multiple", func(t *testing.T) {
		tmp := scaffold(t, map[string]string{
			"github.com/digitalocean/sample-golang":  "v0.0.0-20240904143939-1e058723dcf4",
			"github.com/skyrocknroll/go-mod-example": "v0.0.0-20190130140558-29b3c92445e5",
		})
		require.NoError(t, os.Chdir(tmp))

		require.NoError(t, PinOrchestrion(Options{NoGenerate: true}))

		data, err := parseGoMod(filepath.Join(tmp, "go.mod"))
		require.NoError(t, err)

		assert.NotContains(t, data.Require, goModRequire{"github.com/digitalocean/sample-golang", "v0.0.0-20240904143939-1e058723dcf4"})
		assert.NotContains(t, data.Require, goModRequire{"github.com/skyrocknroll/go-mod-example", "v0.0.0-20190130140558-29b3c92445e5"})
	})

	t.Run("empty-tool-dot-go", func(t *testing.T) {
		tmp := scaffold(t, make(map[string]string))
		require.NoError(t, os.Chdir(tmp))

		toolDotGo := filepath.Join(tmp, config.FilenameOrchestrionToolGo)
		require.NoError(t, os.WriteFile(toolDotGo, nil, 0644))

		require.ErrorContains(t, PinOrchestrion(Options{}), "expected 'package', found 'EOF'")
	})
}

var goModTemplate = template.Must(template.New("go-mod").Parse(`module github.com/DataDog/orchestrion/pin-test

go {{ .GoVersion }}

replace github.com/DataDog/orchestrion {{ .OrchestrionVersion }} => {{ .OrchestrionPath }}

{{ range $path, $version := .Require }}
require	{{ $path }} {{ $version }}
{{ end }}

`))

func scaffold(t *testing.T, requires map[string]string) string {
	t.Helper()
	tmp := t.TempDir()

	_, thisFile, _, _ := runtime.Caller(0)
	rootDir := filepath.Join(thisFile, "..", "..", "..")

	goMod, err := os.Create(filepath.Join(tmp, "go.mod"))
	require.NoError(t, err)

	defer goMod.Close()

	require.NoError(t, goModTemplate.Execute(goMod, struct {
		GoVersion          string
		OrchestrionVersion string
		OrchestrionPath    string
		Require            map[string]string
	}{
		GoVersion:          runtime.Version()[2:6],
		OrchestrionVersion: version.Tag,
		OrchestrionPath:    rootDir,
		Require:            requires,
	}))

	return tmp
}
