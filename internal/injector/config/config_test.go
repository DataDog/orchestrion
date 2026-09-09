// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package config

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/goccy/go-yaml"
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

			hasCfg, err := HasConfig(context.Background(), nil, t.TempDir(), &packages.Package{}, true)
			require.NoError(t, err)
			require.False(t, hasCfg)
		})

		t.Run("ignored files", func(t *testing.T) {
			t.Parallel()

			hasCfg, err := HasConfig(context.Background(), nil, t.TempDir(), &packages.Package{IgnoredFiles: []string{filepath.Join(t.TempDir(), "test.go")}}, true)
			require.NoError(t, err)
			require.False(t, hasCfg)
		})

		t.Run("regular files", func(t *testing.T) {
			t.Parallel()

			hasCfg, err := HasConfig(context.Background(), nil, t.TempDir(), &packages.Package{GoFiles: []string{filepath.Join(t.TempDir(), "test.go")}}, true)
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
			runGo(t, pkgRoot, "mod", "edit", "-replace", "github.com/DataDog/orchestrion="+repoRoot)
			runGo(t, pkgRoot, "mod", "tidy")

			pkg := &packages.Package{
				PkgPath: "github.com/DataDog/orchestrion/config_test",
				GoFiles: []string{filepath.Join(pkgRoot, FilenameOrchestrionToolGo)},
			}
			hasCfg, err := HasConfig(context.Background(), nil, pkgRoot, pkg, true)
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
			hasCfg, err := HasConfig(context.Background(), nil, pkgRoot, pkg, true)
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
			hasCfg, err := HasConfig(context.Background(), nil, pkgRoot, pkg, true)
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
			hasCfg, err := HasConfig(context.Background(), nil, pkgRoot, pkg, false)
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
			_, err := HasConfig(context.Background(), nil, pkgRoot, pkg, true)
			require.ErrorContains(t, err, "meta is required")
		})
	})
}

// TestHasConfigErrorClassification pins the invariant that pruning code
// relies on: HasConfig must only ever report "no configuration" when it
// positively established that, never merely because something failed to
// resolve.
func TestHasConfigErrorClassification(t *testing.T) {
	t.Parallel()

	t.Run("no Go files in the package directory is not an error", func(t *testing.T) {
		t.Parallel()

		pkg := &packages.Package{
			PkgPath: "example.com/x",
			Errors:  []packages.Error{{Kind: packages.ListError, Msg: "no Go files in /x"}},
		}
		hasCfg, err := HasConfig(context.Background(), nil, t.TempDir(), pkg, false)
		require.NoError(t, err)
		require.False(t, hasCfg)
	})

	t.Run("a module that could not be found is an error, not \"no config\"", func(t *testing.T) {
		t.Parallel()

		pkg := &packages.Package{
			PkgPath: "example.com/x",
			Errors:  []packages.Error{{Kind: packages.ListError, Msg: "no required module provides package example.com/x; to add it:\n\tgo get example.com/x"}},
		}
		_, err := HasConfig(context.Background(), nil, t.TempDir(), pkg, false)
		require.Error(t, err)
	})

	t.Run("a broken replace directive is an error, not \"no config\"", func(t *testing.T) {
		t.Parallel()

		// Issue #760: this is the exact shape of the error dd-trace-go's own
		// checkout-relative `replace` directives produce when resolved outside
		// of a dd-trace-go checkout.
		pkg := &packages.Package{
			PkgPath: "example.com/x",
			Errors:  []packages.Error{{Msg: "example.com/x@v1.0.0: replacement directory ../does-not-exist does not exist"}},
		}
		_, err := HasConfig(context.Background(), nil, t.TempDir(), pkg, false)
		require.Error(t, err)
	})
}

// TestHasConfigResolvesFromConsumerModule is a regression test for issue #760:
// a dependency's own go.mod may carry `replace` directives whose paths are
// only valid inside its own checkout (dd-trace-go's `orchestrion/all/v2`
// carries several such directives, e.g. `=> ../../contrib/...`). Import paths
// found in that dependency's orchestrion.tool.go file must be resolved from
// the module that *consumes* the dependency, never from the dependency's own
// directory -- otherwise those checkout-relative replace directives get
// applied and fail to resolve.
func TestHasConfigResolvesFromConsumerModule(t *testing.T) {
	t.Parallel()

	thingDir := t.TempDir()
	runGo(t, thingDir, "mod", "init", "example.com/thing")
	require.NoError(t, os.WriteFile(filepath.Join(thingDir, "thing.go"), []byte(`package thing`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(thingDir, FilenameOrchestrionYML), []byte("meta: {name: name, description: description}\naspects: [{ id: ID, join-point: { package-name: thing }, advice: [add-blank-import: unsafe] }]"), 0o644))

	// dep requires "thing" through a replace directive that is only valid
	// inside dep's own checkout -- mirroring dd-trace-go/orchestrion/all/v2's
	// `replace ... => ../../contrib/...` directives.
	depDir := t.TempDir()
	runGo(t, depDir, "mod", "init", "example.com/dep")
	require.NoError(t, os.WriteFile(filepath.Join(depDir, "dep.go"), []byte(`package dep`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(depDir, FilenameOrchestrionToolGo), []byte(`
		//go:build tools
		package tools
		import _ "example.com/thing"
	`), 0o644))
	runGo(t, depDir, "mod", "edit",
		"-require=example.com/thing@v1.0.0",
		"-replace=example.com/thing@v1.0.0=../does-not-exist",
	)

	// consumer requires "thing" through a valid replace directive, as a real
	// consumer of dep would (via `go mod tidy`, which records a valid
	// resolution for every transitive import reachable through dep's
	// orchestrion.tool.go).
	consumerDir := t.TempDir()
	runGo(t, consumerDir, "mod", "init", "example.com/consumer")
	runGo(t, consumerDir, "mod", "edit",
		"-require=example.com/thing@v1.0.0",
		"-replace=example.com/thing@v1.0.0="+thingDir,
	)

	pkg := &packages.Package{
		PkgPath: "example.com/dep",
		GoFiles: []string{filepath.Join(depDir, "dep.go")},
	}

	t.Run("resolving from the dependency's own directory fails", func(t *testing.T) {
		t.Parallel()

		_, err := HasConfig(context.Background(), nil, depDir, pkg, false)
		require.ErrorContains(t, err, "replacement directory")
	})

	t.Run("resolving from the consuming module succeeds", func(t *testing.T) {
		t.Parallel()

		hasCfg, err := HasConfig(context.Background(), nil, consumerDir, pkg, false)
		require.NoError(t, err)
		require.True(t, hasCfg)
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
		golden.Assert(t, buf.String(), "required.snap.yml")
	})

	t.Run("recursive", func(t *testing.T) {
		tmp := t.TempDir()
		runGo(t, tmp, "mod", "init", "github.com/DataDog/orchestrion/config_test")
		runGo(t, tmp, "mod", "edit", "-replace=github.com/DataDog/orchestrion="+repoRoot)
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
