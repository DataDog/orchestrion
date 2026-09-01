// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package pin

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"text/template"

	"github.com/DataDog/orchestrion/internal/gomod"
	"github.com/DataDog/orchestrion/internal/injector/config"
	"github.com/DataDog/orchestrion/internal/integrations"
	"github.com/DataDog/orchestrion/internal/version"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/mod/semver"
)

func TestPin(t *testing.T) {
	ctx := context.Background()
	if d, ok := t.Deadline(); ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithDeadline(ctx, d)
		defer cancel()
	}

	t.Run("simple", func(t *testing.T) {
		tmp := scaffold(t, make(map[string]string))
		chdir(t, tmp)

		require.NoError(t, PinOrchestrion(ctx, Options{Writer: io.Discard, ErrWriter: io.Discard}))

		assert.FileExists(t, filepath.Join(tmp, config.FilenameOrchestrionToolGo))
		assert.FileExists(t, filepath.Join(tmp, "go.sum"))

		data, err := gomod.Parse(ctx, filepath.Join(tmp, "go.mod"))
		require.NoError(t, err)

		rawTag, _ := version.TagInfo()
		assert.Contains(t, data.Require, gomod.Require{Path: "github.com/DataDog/orchestrion", Version: rawTag})

		content, err := os.ReadFile(filepath.Join(tmp, config.FilenameOrchestrionToolGo))
		require.NoError(t, err)

		assert.Contains(t, string(content), "//go:generate")
	})

	t.Run("upgrade:dd-trace-go", func(t *testing.T) {
		tmp := scaffold(t, map[string]string{integrations.DatadogTracerV1: "v1.73.2"})
		require.NoError(t, os.WriteFile(filepath.Join(tmp, config.FilenameOrchestrionToolGo), []byte(`//go:build tools
package tools

import (
	_ "github.com/DataDog/orchestrion"
	_ "gopkg.in/DataDog/dd-trace-go.v1"
)
`), 0o644))
		// Artificial main.go to retain a reference to "gopkg.in/DataDog/dd-trace-go.v1"
		require.NoError(t, os.WriteFile(filepath.Join(tmp, "main.go"), []byte(`package main

import (
	_ "gopkg.in/DataDog/dd-trace-go.v1"
)

func main() {}
`), 0o644))
		require.NoError(t, gomod.Run(ctx, "tidy", filepath.Join(tmp, "go.mod"), io.Discard))
		chdir(t, tmp)

		// WHEN
		require.NoError(t, PinOrchestrion(ctx, Options{Writer: io.Discard, ErrWriter: io.Discard}))

		// THEN
		data, err := gomod.Parse(ctx, filepath.Join(tmp, "go.mod"))
		require.NoError(t, err)
		ver, found := data.Requires(integrations.DatadogTracerV1)
		assert.True(t, found)
		assert.GreaterOrEqual(t, semver.Compare(ver, "v1.74.0"), 0)

		content, err := os.ReadFile(filepath.Join(tmp, config.FilenameOrchestrionToolGo))
		require.NoError(t, err)
		assert.Contains(t, string(content), integrations.DatadogTracerV2All)
		assert.NotContains(t, string(content), integrations.DatadogTracerV1)
	})

	t.Run("another-version", func(t *testing.T) {
		tmp := scaffold(t, map[string]string{"github.com/DataDog/orchestrion": "v0.9.3"})
		chdir(t, tmp)

		require.NoError(t, PinOrchestrion(ctx, Options{Writer: io.Discard, ErrWriter: io.Discard}))

		assert.FileExists(t, filepath.Join(tmp, config.FilenameOrchestrionToolGo))
		assert.FileExists(t, filepath.Join(tmp, "go.sum"))

		data, err := gomod.Parse(ctx, filepath.Join(tmp, "go.mod"))
		require.NoError(t, err)

		rawTag, _ := version.TagInfo()
		assert.Contains(t, data.Require, gomod.Require{Path: "github.com/DataDog/orchestrion", Version: rawTag})
	})

	t.Run("no-generate", func(t *testing.T) {
		tmp := scaffold(t, make(map[string]string))
		chdir(t, tmp)

		require.NoError(t, PinOrchestrion(ctx, Options{Writer: io.Discard, ErrWriter: io.Discard, NoGenerate: true}))

		content, err := os.ReadFile(filepath.Join(tmp, config.FilenameOrchestrionToolGo))
		require.NoError(t, err)

		assert.NotContains(t, string(content), "//go:generate")
	})

	t.Run("prune", func(t *testing.T) {
		tmp := scaffold(t, map[string]string{"github.com/digitalocean/sample-golang": "v0.0.0-20240904143939-1e058723dcf4"})
		chdir(t, tmp)

		require.NoError(t, PinOrchestrion(ctx, Options{Writer: io.Discard, ErrWriter: io.Discard, NoGenerate: true}))

		data, err := gomod.Parse(ctx, filepath.Join(tmp, "go.mod"))
		require.NoError(t, err)

		assert.NotContains(t, data.Require, gomod.Require{Path: "github.com/digitalocean/sample-golang", Version: "v0.0.0-20240904143939-1e058723dcf4"})
	})

	t.Run("prune-multiple", func(t *testing.T) {
		tmp := scaffold(t, map[string]string{
			"github.com/digitalocean/sample-golang":  "v0.0.0-20240904143939-1e058723dcf4",
			"github.com/skyrocknroll/go-mod-example": "v0.0.0-20190130140558-29b3c92445e5",
		})
		chdir(t, tmp)

		require.NoError(t, PinOrchestrion(ctx, Options{Writer: io.Discard, ErrWriter: io.Discard, NoGenerate: true}))

		data, err := gomod.Parse(ctx, filepath.Join(tmp, "go.mod"))
		require.NoError(t, err)

		assert.NotContains(t, data.Require, gomod.Require{Path: "github.com/digitalocean/sample-golang", Version: "v0.0.0-20240904143939-1e058723dcf4"})
		assert.NotContains(t, data.Require, gomod.Require{Path: "github.com/skyrocknroll/go-mod-example", Version: "v0.0.0-20190130140558-29b3c92445e5"})
	})

	t.Run("workspace-mode", func(t *testing.T) {
		// Regression test: `-mod` may only be `readonly` or `vendor` while in
		// workspace mode, so pruneImports's package resolution must disable
		// workspace mode (GOWORK=off) rather than unconditionally forcing
		// `-mod=mod`, or this fails for any project governed by a `go.work`
		// file (e.g. etcd-style monorepos).
		tmp := scaffold(t, make(map[string]string))
		require.NoError(t, os.WriteFile(filepath.Join(tmp, "go.work"), []byte("go "+runtime.Version()[2:6]+".0\n\nuse .\n"), 0o644))
		chdir(t, tmp)

		require.NoError(t, PinOrchestrion(ctx, Options{Writer: io.Discard, ErrWriter: io.Discard}))

		assert.FileExists(t, filepath.Join(tmp, config.FilenameOrchestrionToolGo))
	})

	t.Run("vendor-inconsistent-after-integration-install", func(t *testing.T) {
		// Regression test for https://github.com/DataDog/orchestrion/issues/687:
		// `orchestrion pin` must not fail with "inconsistent vendoring" when
		// `github.com/DataDog/orchestrion` is already required in `go.mod` (e.g.
		// via a prior `go get`) but `vendor/` has not been re-synced yet, and
		// installing the dd-trace-go integration mutates `go.mod` directly.
		tmp := scaffold(t, map[string]string{"github.com/google/uuid": "v1.6.0"})
		require.NoError(t, os.WriteFile(filepath.Join(tmp, "main.go"), []byte(`package main

import (
	"fmt"

	"github.com/google/uuid"
)

func main() { fmt.Println(uuid.NewString()) }
`), 0o644))

		// `go mod vendor` writes `vendor/` relative to the process's working
		// directory (unlike `go mod edit`/`go mod tidy`, which only touch the
		// designated `-modfile`), so we must chdir before invoking it.
		chdir(t, tmp)

		modfile := filepath.Join(tmp, "go.mod")
		require.NoError(t, gomod.Run(ctx, "tidy", modfile, io.Discard))
		require.NoError(t, gomod.Run(ctx, "vendor", modfile, io.Discard))

		// Simulate `github.com/DataDog/orchestrion` already being required (e.g.
		// by a prior `go get`) without `vendor/` having been re-synced yet.
		rawTag, _ := version.TagInfo()
		require.NoError(t, gomod.Run(ctx, "edit", modfile, io.Discard, "-require=github.com/DataDog/orchestrion@"+rawTag))

		// WHEN
		require.NoError(t, PinOrchestrion(ctx, Options{Writer: io.Discard, ErrWriter: io.Discard}))

		// THEN: `vendor/` must be left consistent with `go.mod`, or the `go
		// build -mod vendor` that normally follows `orchestrion pin` would fail
		// with the exact "inconsistent vendoring" error reported in #687.
		cmd := exec.CommandContext(ctx, "go", "list", "-mod=vendor", "./...")
		cmd.Dir = tmp
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, string(out))
	})

	t.Run("prune-persists-to-tool-file", func(t *testing.T) {
		// Regression test: pruneImports only mutated the in-memory AST; nothing
		// ever wrote it back to `orchestrion.tool.go`, so a manually-added import
		// to a package with no `orchestrion.yml` was silently kept on disk even
		// though `go.mod` correctly dropped the now-unused requirement.
		tmp := scaffold(t, map[string]string{"github.com/digitalocean/sample-golang": "v0.0.0-20240904143939-1e058723dcf4"})
		require.NoError(t, os.WriteFile(filepath.Join(tmp, config.FilenameOrchestrionToolGo), []byte(`//go:build tools
package tools

import (
	_ "github.com/DataDog/orchestrion"
	_ "github.com/digitalocean/sample-golang" // integration
)
`), 0o644))
		chdir(t, tmp)

		require.NoError(t, PinOrchestrion(ctx, Options{Writer: io.Discard, ErrWriter: io.Discard, NoGenerate: true}))

		content, err := os.ReadFile(filepath.Join(tmp, config.FilenameOrchestrionToolGo))
		require.NoError(t, err)
		assert.NotContains(t, string(content), "github.com/digitalocean/sample-golang")

		data, err := gomod.Parse(ctx, filepath.Join(tmp, "go.mod"))
		require.NoError(t, err)
		assert.NotContains(t, data.Require, gomod.Require{Path: "github.com/digitalocean/sample-golang", Version: "v0.0.0-20240904143939-1e058723dcf4"})
	})

	t.Run("no-prune-clears-marker-comment", func(t *testing.T) {
		// Regression test: on the `-prune=false` path, `pruneImport` clears the
		// `// integration` trailing comment on flagged imports (without removing
		// them), but that mutation was never persisted either.
		tmp := scaffold(t, map[string]string{"github.com/digitalocean/sample-golang": "v0.0.0-20240904143939-1e058723dcf4"})
		require.NoError(t, os.WriteFile(filepath.Join(tmp, config.FilenameOrchestrionToolGo), []byte(`//go:build tools
package tools

import (
	_ "github.com/DataDog/orchestrion"
	_ "github.com/digitalocean/sample-golang" // integration
)
`), 0o644))
		chdir(t, tmp)

		require.NoError(t, PinOrchestrion(ctx, Options{Writer: io.Discard, ErrWriter: io.Discard, NoGenerate: true, NoPrune: true}))

		content, err := os.ReadFile(filepath.Join(tmp, config.FilenameOrchestrionToolGo))
		require.NoError(t, err)

		found := false
		for _, line := range strings.Split(string(content), "\n") {
			if !strings.Contains(line, "github.com/digitalocean/sample-golang") {
				continue
			}
			found = true
			assert.NotContains(t, line, "integration", "the `// integration` marker should have been cleared, not just the in-memory copy")
		}
		assert.True(t, found, "the unnecessary import should still be present, since -prune=false only warns")
	})

	t.Run("empty-tool-dot-go", func(t *testing.T) {
		tmp := scaffold(t, make(map[string]string))
		chdir(t, tmp)

		toolDotGo := filepath.Join(tmp, config.FilenameOrchestrionToolGo)
		require.NoError(t, os.WriteFile(toolDotGo, nil, 0644))

		require.ErrorContains(t, PinOrchestrion(ctx, Options{Writer: io.Discard, ErrWriter: io.Discard}), "expected 'package', found 'EOF'")
	})
}

var goModTemplate = template.Must(template.New("go-mod").Parse(`module github.com/DataDog/orchestrion/pin-test

go {{ .GoVersion }}

replace (
	github.com/DataDog/orchestrion {{ .OrchestrionVersion }} => {{ .OrchestrionPath }}
)

require (
{{- range $path, $version := .Require }}
	{{ $path }} {{ $version }}
{{- end }}
)
`))

func chdir(t *testing.T, dir string) {
	t.Helper()

	oldwd, err := os.Getwd()
	require.NoError(t, err)

	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { require.NoError(t, os.Chdir(oldwd)) })
}

func scaffold(t *testing.T, requires map[string]string) string {
	t.Helper()
	tmp := t.TempDir()

	_, thisFile, _, _ := runtime.Caller(0)
	rootDir := filepath.Join(thisFile, "..", "..", "..")

	goMod, err := os.Create(filepath.Join(tmp, "go.mod"))
	require.NoError(t, err)

	defer goMod.Close()

	rawTag, _ := version.TagInfo()
	require.NoError(t, goModTemplate.Execute(goMod, struct {
		GoVersion          string
		OrchestrionVersion string
		OrchestrionPath    string
		PathSep            string
		Require            map[string]string
	}{
		GoVersion:          runtime.Version()[2:6],
		OrchestrionVersion: rawTag,
		OrchestrionPath:    rootDir,
		PathSep:            string(filepath.Separator),
		Require:            requires,
	}))

	return tmp
}
