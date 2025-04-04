// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package config

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/goccy/go-yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/packages"
	"gotest.tools/v3/golden"
)

func TestHasConfig(t *testing.T) {
	t.Parallel()

	t.Run("no configuration", func(t *testing.T) {
		t.Parallel()

		t.Run("no source files at all", func(t *testing.T) {
			t.Parallel()

			hasCfg, err := HasConfig(context.Background(), nil, &packages.Package{}, true)
			require.NoError(t, err)
			require.False(t, hasCfg)
		})

		t.Run("ignored files", func(t *testing.T) {
			t.Parallel()

			hasCfg, err := HasConfig(context.Background(), nil, &packages.Package{IgnoredFiles: []string{filepath.Join(t.TempDir(), "test.go")}}, true)
			require.NoError(t, err)
			require.False(t, hasCfg)
		})

		t.Run("regular files", func(t *testing.T) {
			t.Parallel()

			hasCfg, err := HasConfig(context.Background(), nil, &packages.Package{GoFiles: []string{filepath.Join(t.TempDir(), "test.go")}}, true)
			require.NoError(t, err)
			require.False(t, hasCfg)
		})
	})

	t.Run("configuration", func(t *testing.T) {
		t.Parallel()

		_, thisFile, _, _ := runtime.Caller(0)
		repoRoot := filepath.Join(thisFile, "..", "..", "..", "..")

		t.Run("only "+FilenameOrchestrionToolGo, func(t *testing.T) {
			t.Parallel()

			pkgRoot := t.TempDir()
			runGo(t, pkgRoot, "mod", "init", "github.com/DataDog/orchestrion/config_test")
			require.NoError(t, os.WriteFile(filepath.Join(pkgRoot, FilenameOrchestrionToolGo), []byte(`
				//go:build tools
				package tools
				import _ "github.com/DataDog/orchestrion"
			`), 0o644))
			runGo(t, pkgRoot, "mod", "edit", "-replace", fmt.Sprintf("github.com/DataDog/orchestrion=%s", repoRoot))
			runGo(t, pkgRoot, "mod", "tidy")

			pkg := &packages.Package{
				PkgPath: "github.com/DataDog/orchestrion/config_test",
				GoFiles: []string{filepath.Join(pkgRoot, FilenameOrchestrionToolGo)},
			}
			hasCfg, err := HasConfig(context.Background(), nil, pkg, true)
			require.NoError(t, err)
			require.True(t, hasCfg)
		})

		t.Run("only "+FilenameOrchestrionYML, func(t *testing.T) {
			t.Parallel()

			pkgRoot := t.TempDir()
			runGo(t, pkgRoot, "mod", "init", "github.com/DataDog/orchestrion/config_test")
			require.NoError(t, os.WriteFile(filepath.Join(pkgRoot, FilenameOrchestrionYML), []byte("meta: {name: name, description: description}\naspects: [{ id: ID, join-point: { package-name: main }, advice: [add-blank-import: unsafe] }]"), 0o644))

			pkg := &packages.Package{
				PkgPath: "github.com/DataDog/orchestrion/config_test",
				GoFiles: []string{filepath.Join(pkgRoot, "main.go")},
			}
			hasCfg, err := HasConfig(context.Background(), nil, pkg, true)
			require.NoError(t, err)
			require.True(t, hasCfg)
		})

		t.Run("complete", func(t *testing.T) {
			t.Parallel()

			pkgRoot := t.TempDir()
			runGo(t, pkgRoot, "mod", "init", "github.com/DataDog/orchestrion/config_test")
			require.NoError(t, os.WriteFile(filepath.Join(pkgRoot, FilenameOrchestrionToolGo), []byte(`
				//go:build tools
				package tools
				import _ "github.com/DataDog/orchestrion/config_test/inner"
			`), 0o644))
			require.NoError(t, os.Mkdir(filepath.Join(pkgRoot, "inner"), 0o755))
			require.NoError(t, os.WriteFile(filepath.Join(pkgRoot, "inner", "inner.go"), []byte(`package inner`), 0o644))
			require.NoError(t, os.WriteFile(filepath.Join(pkgRoot, "inner", FilenameOrchestrionYML), []byte("meta: {name: name, description: description}\naspects: [{ id: ID2, join-point: { package-name: inner }, advice: [add-blank-import: unsafe] }]"), 0o644))
			require.NoError(t, os.WriteFile(filepath.Join(pkgRoot, FilenameOrchestrionYML), []byte("meta: {name: name, description: description}\naspects: [{ id: ID, join-point: { package-name: main }, advice: [add-blank-import: unsafe] }]"), 0o644))

			pkg := &packages.Package{
				PkgPath: "github.com/DataDog/orchestrion/config_test",
				GoFiles: []string{filepath.Join(pkgRoot, FilenameOrchestrionToolGo)},
			}
			hasCfg, err := HasConfig(context.Background(), nil, pkg, true)
			require.NoError(t, err)
			require.True(t, hasCfg)
		})

		t.Run("invalid (not validating)", func(t *testing.T) {
			t.Parallel()

			pkgRoot := t.TempDir()
			runGo(t, pkgRoot, "mod", "init", "github.com/DataDog/orchestrion/config_test")
			require.NoError(t, os.WriteFile(filepath.Join(pkgRoot, FilenameOrchestrionToolGo), []byte(`
				//go:build tools
				package tools
				import _ "github.com/DataDog/orchestrion/config_test/inner"
			`), 0o644))
			require.NoError(t, os.Mkdir(filepath.Join(pkgRoot, "inner"), 0o755))
			require.NoError(t, os.WriteFile(filepath.Join(pkgRoot, "inner", "inner.go"), []byte(`package inner`), 0o644))
			// Invalid -- there is no "meta" block in there...
			require.NoError(t, os.WriteFile(filepath.Join(pkgRoot, "inner", FilenameOrchestrionYML), []byte("aspects: [{ id: ID2, join-point: { package-name: inner }, advice: [add-blank-import: unsafe] }]"), 0o644))
			require.NoError(t, os.WriteFile(filepath.Join(pkgRoot, FilenameOrchestrionYML), []byte("meta: {name: name, description: description}\naspects: [{ id: ID, join-point: { package-name: main }, advice: [add-blank-import: unsafe] }]"), 0o644))

			pkg := &packages.Package{
				PkgPath: "github.com/DataDog/orchestrion/config_test",
				GoFiles: []string{filepath.Join(pkgRoot, FilenameOrchestrionToolGo)},
			}
			hasCfg, err := HasConfig(context.Background(), nil, pkg, false)
			require.NoError(t, err)
			require.True(t, hasCfg)
		})

		t.Run("invalid (validating)", func(t *testing.T) {
			t.Parallel()

			pkgRoot := t.TempDir()
			runGo(t, pkgRoot, "mod", "init", "github.com/DataDog/orchestrion/config_test")
			require.NoError(t, os.WriteFile(filepath.Join(pkgRoot, FilenameOrchestrionToolGo), []byte(`
				//go:build tools
				package tools
				import _ "github.com/DataDog/orchestrion/config_test/inner"
			`), 0o644))
			require.NoError(t, os.Mkdir(filepath.Join(pkgRoot, "inner"), 0o755))
			require.NoError(t, os.WriteFile(filepath.Join(pkgRoot, "inner", "inner.go"), []byte(`package inner`), 0o644))
			// Invalid -- there is no "meta" block in there...
			require.NoError(t, os.WriteFile(filepath.Join(pkgRoot, "inner", FilenameOrchestrionYML), []byte("aspects: [{ id: ID2, join-point: { package-name: inner }, advice: [add-blank-import: unsafe] }]"), 0o644))
			require.NoError(t, os.WriteFile(filepath.Join(pkgRoot, FilenameOrchestrionYML), []byte("meta: {name: name, description: description}\naspects: [{ id: ID, join-point: { package-name: main }, advice: [add-blank-import: unsafe] }]"), 0o644))

			pkg := &packages.Package{
				PkgPath: "github.com/DataDog/orchestrion/config_test",
				GoFiles: []string{filepath.Join(pkgRoot, FilenameOrchestrionToolGo)},
			}
			_, err := HasConfig(context.Background(), nil, pkg, true)
			require.ErrorContains(t, err, "meta is required")
		})
	})
}

func TestLoad(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	repoRoot := filepath.Join(thisFile, "..", "..", "..", "..")

	t.Run("no go files", func(t *testing.T) {
		tmpDir := t.TempDir()
		goMod := filepath.Join(tmpDir, "go.mod")
		require.NoError(t, os.WriteFile(goMod, []byte("module test\n"), 0o644))
		loader := NewLoader(nil, tmpDir, true)
		_, err := loader.Load(context.Background())
		require.ErrorContains(t, err, "no Go files found, was expecting at least orchestrion.tool.go")
	})

	t.Run("required.yml", func(t *testing.T) {
		loader := NewLoader(nil, repoRoot, true)
		cfg, err := loader.Load(context.Background())
		require.NoError(t, err)

		var buf bytes.Buffer
		enc := yaml.NewEncoder(&buf, yaml.Indent(2), yaml.IndentSequence(true), yaml.UseSingleQuote(true))
		defer func() { require.NoError(t, enc.Close()) }()
		require.NoError(t, enc.Encode(cfg))

		assert.Len(t, cfg.Aspects(), len(builtIn.yaml.aspects)) // Just the orchestrion:... aspects
		golden.Assert(t, buf.String(), "required.snap.yml")
	})

	t.Run("instrument", func(t *testing.T) {
		loader := NewLoader(nil, filepath.Join(repoRoot, "instrument"), true)
		cfg, err := loader.Load(context.Background())
		require.NoError(t, err)

		var buf bytes.Buffer
		enc := yaml.NewEncoder(&buf, yaml.Indent(2), yaml.IndentSequence(true), yaml.UseSingleQuote(true))
		defer func() { require.NoError(t, enc.Close()) }()
		require.NoError(t, enc.Encode(cfg))

		assert.Len(t, cfg.Aspects(), 115+len(builtIn.yaml.aspects))
		golden.Assert(t, buf.String(), "instrument.snap.yml")
	})

	t.Run("recursive", func(t *testing.T) {
		tmp := t.TempDir()
		runGo(t, tmp, "mod", "init", "github.com/DataDog/orchestrion/config_test")
		runGo(t, tmp, "mod", "edit", fmt.Sprintf("-replace=github.com/DataDog/orchestrion=%s", repoRoot))
		require.NoError(t, os.WriteFile(filepath.Join(tmp, FilenameOrchestrionToolGo), []byte(`
			//go:build tools
			package tools
			import _ "github.com/DataDog/orchestrion/config_test/nested"
		`), 0o644))
		require.NoError(t, os.Mkdir(filepath.Join(tmp, "nested"), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(tmp, "nested", "nested.go"), []byte(`package nested`), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(tmp, "nested", FilenameOrchestrionYML), []byte(`extends: [../sibling]`), 0o644))
		require.NoError(t, os.Mkdir(filepath.Join(tmp, "sibling"), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(tmp, "sibling", FilenameOrchestrionYML), []byte(`aspects: [{ id: "ID", join-point: { package-name: main }, advice: [{ add-blank-import: unsafe }] }]`), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(tmp, "sibling", FilenameOrchestrionToolGo), []byte(`
			//go:build tools
			package tools
			import (
				_ "github.com/DataDog/orchestrion"
				_ "github.com/DataDog/orchestrion/config_test"
			)
		`), 0o644))
		runGo(t, tmp, "mod", "tidy")

		loader := NewLoader(nil, tmp, false)
		cfg, err := loader.Load(context.Background())
		require.NoError(t, err)
		require.Len(t, cfg.Aspects(), len(builtIn.yaml.aspects)+1)
	})
}

func runGo(t *testing.T, tmp string, args ...string) {
	cmd := exec.Command("go", args...)
	cmd.Dir = tmp
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	require.NoError(t, cmd.Run(), "running: go %v", args)
}

var (
	_ yaml.InterfaceMarshaler = (*configGo)(nil)
	_ yaml.InterfaceMarshaler = (*configYML)(nil)
)

func (c *configGo) MarshalYAML() (any, error) {
	type print struct {
		PkgPath string
		Imports []Config `yaml:",omitempty"`
		YAML    Config   `yaml:",omitempty"`
	}
	return print{c.pkgPath, c.imports, c.yaml}, nil
}

func (c *configYML) MarshalYAML() (any, error) {
	if c == nil {
		return nil, nil
	}

	type print struct {
		Name    string
		Extends []Config `yaml:",omitempty"`
		Aspects []string `yaml:",omitempty"`
	}
	aspects := make([]string, len(c.aspects))
	for i, a := range c.aspects {
		aspects[i] = a.ID
	}
	return print{c.name, c.extends, aspects}, nil
}
