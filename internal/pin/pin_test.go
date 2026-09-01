// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package pin

import (
	"bytes"
	"context"
	"io"
	"os"
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

		var out, errOut bytes.Buffer
		require.NoError(t, PinOrchestrion(ctx, Options{Writer: &out, ErrWriter: &errOut}))

		assert.FileExists(t, filepath.Join(tmp, config.FilenameOrchestrionToolGo))
		assert.FileExists(t, filepath.Join(tmp, "go.sum"))

		data, err := gomod.Parse(ctx, filepath.Join(tmp, "go.mod"))
		require.NoError(t, err)

		rawTag, _ := version.TagInfo()
		assert.Contains(t, data.Require, gomod.Require{Path: "github.com/DataDog/orchestrion", Version: rawTag})

		content, err := os.ReadFile(filepath.Join(tmp, config.FilenameOrchestrionToolGo))
		require.NoError(t, err)

		assert.Contains(t, string(content), "//go:generate")

		// Regression test for issue #760: dd-trace-go/orchestrion/all/v2 must
		// never be reported as an unnecessary import, and its own transitive
		// import resolution failures (which happen when its own
		// checkout-relative `replace` directives are mistakenly applied) must
		// never surface as a warning either.
		assert.Contains(t, string(content), integrations.DatadogTracerV2All)
		assert.NotContains(t, out.String(), "unnecessary import")
		assert.NotContains(t, errOut.String(), "note: keeping")
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

	// These two tests exercise `go mod tidy` removing an unused require, not
	// pruneImports: the extra modules are only added to go.mod's requires, and
	// are never imported from orchestrion.tool.go, so pruneImports never even
	// considers them. Real prune coverage lives in
	// "does not falsely prune an import whose transitive replace is
	// checkout-relative" below and in internal/injector/config.

	t.Run("tidy:removes-unused-require", func(t *testing.T) {
		tmp := scaffold(t, map[string]string{"github.com/digitalocean/sample-golang": "v0.0.0-20240904143939-1e058723dcf4"})
		chdir(t, tmp)

		require.NoError(t, PinOrchestrion(ctx, Options{Writer: io.Discard, ErrWriter: io.Discard, NoGenerate: true}))

		data, err := gomod.Parse(ctx, filepath.Join(tmp, "go.mod"))
		require.NoError(t, err)

		assert.NotContains(t, data.Require, gomod.Require{Path: "github.com/digitalocean/sample-golang", Version: "v0.0.0-20240904143939-1e058723dcf4"})
	})

	t.Run("tidy:removes-multiple-unused-requires", func(t *testing.T) {
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

	t.Run("does not falsely prune an import whose transitive replace is checkout-relative", func(t *testing.T) {
		// Regression test for issue #760: a dependency's own go.mod may carry
		// `replace` directives whose paths are only valid inside its own
		// checkout (dd-trace-go/orchestrion/all/v2 carries several such
		// directives, e.g. `=> ../../contrib/...`). Import paths found in that
		// dependency's orchestrion.tool.go file must be resolved from the
		// module that consumes it, never from the dependency's own directory --
		// otherwise those checkout-relative replace directives get applied and
		// fail to resolve, and the import gets misreported as unnecessary.
		tmp := scaffold(t, make(map[string]string))
		chdir(t, tmp)

		_, thisFile, _, _ := runtime.Caller(0)
		fixtureRoot := filepath.Join(thisFile, "..", "testdata", "relative-replace")
		depDir := filepath.Join(fixtureRoot, "dep")
		thingDir := filepath.Join(fixtureRoot, "thing")

		require.NoError(t, gomod.Run(ctx, "edit", filepath.Join(tmp, "go.mod"), io.Discard,
			"-require=example.com/dep@v1.0.0",
			"-replace=example.com/dep@v1.0.0="+depDir,
			"-require=example.com/thing@v1.0.0",
			"-replace=example.com/thing@v1.0.0="+thingDir,
		))

		require.NoError(t, os.WriteFile(filepath.Join(tmp, config.FilenameOrchestrionToolGo), []byte(`//go:build tools
package tools

import (
	_ "github.com/DataDog/orchestrion"
	_ "example.com/dep"
)
`), 0o644))

		var out, errOut bytes.Buffer
		require.NoError(t, PinOrchestrion(ctx, Options{Writer: &out, ErrWriter: &errOut, NoGenerate: true}))

		assert.NotContains(t, out.String(), "unnecessary import")
		assert.NotContains(t, errOut.String(), "note: keeping")

		content, err := os.ReadFile(filepath.Join(tmp, config.FilenameOrchestrionToolGo))
		require.NoError(t, err)
		assert.Contains(t, string(content), `"example.com/dep"`)
	})

	t.Run("empty-tool-dot-go", func(t *testing.T) {
		tmp := scaffold(t, make(map[string]string))
		chdir(t, tmp)

		toolDotGo := filepath.Join(tmp, config.FilenameOrchestrionToolGo)
		require.NoError(t, os.WriteFile(toolDotGo, nil, 0644))

		require.ErrorContains(t, PinOrchestrion(ctx, Options{Writer: io.Discard, ErrWriter: io.Discard}), "expected 'package', found 'EOF'")
	})
}

// TestPruneImportsLoadError verifies that an import that fails to load (as
// opposed to one that loads fine but lacks orchestrion configuration) is
// left untouched with a non-fatal warning on ErrWriter, using the actual
// load error as the reason -- a resolution failure is not evidence that the
// package provides no orchestrion integrations, so it must never be pruned.
//
// This is exercised by calling pruneImports directly instead of going
// through PinOrchestrion: PinOrchestrion runs `go mod tidy` before
// pruneImports ever runs, and `go mod tidy` already fails hard on an
// unresolvable import, so the pkg.Errors branch inside pruneImports is not
// reachable from that entry point in this scenario.
func TestPruneImportsLoadError(t *testing.T) {
	ctx := context.Background()

	tmp := scaffold(t, make(map[string]string))
	chdir(t, tmp)

	toolDotGo := filepath.Join(tmp, config.FilenameOrchestrionToolGo)
	require.NoError(t, os.WriteFile(toolDotGo, []byte(`//go:build tools
package tools

import (
	_ "github.com/DataDog/orchestrion"
	_ "github.com/DataDog/orchestrion/this-package-does-not-exist-zzz"
)
`), 0o644))

	dstFile, err := parseOrchestrionToolGo(toolDotGo)
	require.NoError(t, err)
	importSet := importSetFrom(dstFile)

	var out, errOut bytes.Buffer
	pruned, err := pruneImports(ctx, tmp, importSet, Options{Writer: &out, ErrWriter: &errOut})
	require.NoError(t, err)

	assert.False(t, pruned)
	assert.NotNil(t, importSet.Find("github.com/DataDog/orchestrion/this-package-does-not-exist-zzz"))
	assert.Contains(t, errOut.String(), "note: keeping")
	assert.NotContains(t, out.String(), "there is no "+config.FilenameOrchestrionYML)
}

// TestPruneImportsBuildTagExcluded verifies that an import whose only Go file
// is excluded by build tags (so packages.Load reports pkg.Errors alongside
// pkg.IgnoredFiles) is not pruned when it has an adjacent orchestrion.tool.go,
// since config.HasConfig locates configuration via pkg.IgnoredFiles in that
// case. Reported by Codex during review of PR #891.
func TestPruneImportsBuildTagExcluded(t *testing.T) {
	ctx := context.Background()

	tmp := scaffold(t, make(map[string]string))
	chdir(t, tmp)

	pkgDir := filepath.Join(tmp, "buildtagpkg")
	require.NoError(t, os.MkdirAll(pkgDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(pkgDir, "only_tag.go"), []byte(`//go:build orchestrion_never_defined

package buildtagpkg
`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(pkgDir, config.FilenameOrchestrionToolGo), []byte(`//go:build tools
package tools

import (
	_ "github.com/DataDog/orchestrion"
)
`), 0o644))

	toolDotGo := filepath.Join(tmp, config.FilenameOrchestrionToolGo)
	require.NoError(t, os.WriteFile(toolDotGo, []byte(`//go:build tools
package tools

import (
	_ "github.com/DataDog/orchestrion"
	_ "github.com/DataDog/orchestrion/pin-test/buildtagpkg"
)
`), 0o644))

	dstFile, err := parseOrchestrionToolGo(toolDotGo)
	require.NoError(t, err)
	importSet := importSetFrom(dstFile)

	var out bytes.Buffer
	pruned, err := pruneImports(ctx, tmp, importSet, Options{Writer: &out, ErrWriter: io.Discard})
	require.NoError(t, err)

	assert.False(t, pruned)
	assert.NotNil(t, importSet.Find("github.com/DataDog/orchestrion/pin-test/buildtagpkg"))
	assert.Empty(t, out.String())
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
