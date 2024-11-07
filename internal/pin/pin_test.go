// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package pin

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"text/template"

	"github.com/DataDog/orchestrion/internal/version"
	"github.com/stretchr/testify/require"
)

func TestPin(t *testing.T) {
	if cwd, err := os.Getwd(); err == nil {
		defer require.NoError(t, os.Chdir(cwd))
	}

	tmp := scaffold(t, make(map[string]string))
	require.NoError(t, os.Chdir(tmp))
	AutoPinOrchestrion()
	require.NotEmpty(t, os.Getenv(envVarCheckedGoMod))

	require.FileExists(t, filepath.Join(tmp, orchestrionToolGo))
	require.FileExists(t, filepath.Join(tmp, "go.sum"))

	data, err := os.ReadFile(filepath.Join(tmp, "go.mod"))
	require.NoError(t, err)
	require.Contains(t, string(data), fmt.Sprintf(`github.com/DataDog/orchestrion %s`, version.Tag))
}

var goModTemplate = template.Must(template.New("go-mod").Parse(`module github.com/DataDog/orchestrion/pin-test

go {{ .GoVersion }}

replace github.com/DataDog/orchestrion {{ .OrchestrionVersion }} => {{ .OrchestrionPath }}

require (
{{ range $path, $version := .Require }}
	{{ $path }} {{ $version }}
{{ end }}
)
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
