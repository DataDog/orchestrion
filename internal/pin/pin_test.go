// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package pin

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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
		// workspace mode, so pruneImports's package resolution must not
		// unconditionally force `-mod=mod`, or this fails for any project
		// governed by a `go.work` file (e.g. etcd-style monorepos).
		tmp := scaffold(t, make(map[string]string))
		require.NoError(t, os.WriteFile(filepath.Join(tmp, "go.work"), []byte("go "+runtime.Version()[2:6]+".0\n\nuse .\n"), 0o644))
		chdir(t, tmp)

		require.NoError(t, PinOrchestrion(ctx, Options{Writer: io.Discard, ErrWriter: io.Discard}))

		assert.FileExists(t, filepath.Join(tmp, config.FilenameOrchestrionToolGo))
	})

	t.Run("workspace-mode-honors-replace", func(t *testing.T) {
		// Regression test: pruneImports's package resolution must not disable
		// workspace mode (e.g. via GOWORK=off) to work around the `-mod`
		// restriction above, or it would resolve a `go.work`-replaced module
		// from its plain `go.mod`-replaced (or published) copy instead of the
		// workspace's own version — silently pruning a real integration whose
		// configuration only exists in the workspace copy. Per `go help work`,
		// workspace replacements take precedence over `go.mod` replacements.
		tmp := t.TempDir()
		mainDir := filepath.Join(tmp, "main")
		fakeA := filepath.Join(tmp, "fakeA") // plain go.mod replace target: no config
		fakeB := filepath.Join(tmp, "fakeB") // go.work replace target: has config
		for _, dir := range []string{mainDir, fakeA, fakeB} {
			require.NoError(t, os.MkdirAll(dir, 0o755))
		}

		goVersion := runtime.Version()[2:6]
		const fakeModule = "example.com/fakeintegration"

		require.NoError(t, os.WriteFile(filepath.Join(fakeA, "go.mod"), []byte("module "+fakeModule+"\n\ngo "+goVersion+".0\n"), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(fakeA, "pkg.go"), []byte("package fakeintegration\n"), 0o644))

		require.NoError(t, os.WriteFile(filepath.Join(fakeB, "go.mod"), []byte("module "+fakeModule+"\n\ngo "+goVersion+".0\n"), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(fakeB, "pkg.go"), []byte("package fakeintegration\n"), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(fakeB, config.FilenameOrchestrionYML), []byte(
			"meta: {name: name, description: description}\n"+
				"aspects: [{ id: ID, join-point: { package-name: fakeintegration }, advice: [add-blank-import: unsafe] }]",
		), 0o644))

		_, thisFile, _, _ := runtime.Caller(0)
		rootDir := filepath.Join(thisFile, "..", "..", "..")
		rawTag, _ := version.TagInfo()
		require.NoError(t, os.WriteFile(filepath.Join(mainDir, "go.mod"), []byte(fmt.Sprintf(`module example.com/wsmain

go %s.0

replace (
	github.com/DataDog/orchestrion %s => %s
	%s => ../fakeA
)

require (
	github.com/DataDog/orchestrion %[2]s
	%[4]s v0.0.0-00010101000000-000000000000
)
`, goVersion, rawTag, rootDir, fakeModule)), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(mainDir, config.FilenameOrchestrionToolGo), []byte(`//go:build tools

package tools

import (
	_ "github.com/DataDog/orchestrion"
	_ "`+fakeModule+`"
)
`), 0o644))

		require.NoError(t, os.WriteFile(filepath.Join(tmp, "go.work"), []byte(fmt.Sprintf(`go %s.0

use ./main

replace %s => ./fakeB
`, goVersion, fakeModule)), 0o644))

		chdir(t, mainDir)

		// The prune decision is what we're testing, not the resulting file: a
		// separate, pre-existing bug means a pruned import's removal is never
		// persisted back to `orchestrion.tool.go` (it's already been written to
		// disk before pruning runs), so we must observe the prune decision via
		// the message `pruneImport` prints, not by re-reading the tool file.
		var out bytes.Buffer
		require.NoError(t, PinOrchestrion(ctx, Options{Writer: &out, ErrWriter: io.Discard}))

		assert.NotContains(t, out.String(), fakeModule, "the workspace-replaced (configured) copy must win, so this import must not be pruned")
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
