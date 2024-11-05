// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package config

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/DataDog/orchestrion/internal/pin"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
	"gotest.tools/v3/golden"
)

func TestLoad(t *testing.T) {
	t.Run("samples", func(t *testing.T) {
		samples := filepath.Join(rootDir, "samples")

		cfg, err := Load(samples, true)
		require.NoError(t, err)
		require.NotNil(t, cfg)

		var yml bytes.Buffer
		enc := yaml.NewEncoder(&yml)
		require.NoError(t, enc.Encode(cfg))
		require.NoError(t, enc.Close())
		golden.AssertBytes(t, yml.Bytes(), "samples.snap.yml")

		require.Equal(t, countAspects(t, cfg), len(cfg.Aspects()))
	})

	t.Run("circular", func(t *testing.T) {
		tmp := t.TempDir()

		run(t, tmp, "go", "mod", "init", "github.com/DataDog/orchestrion/test.circular")
		run(t, tmp, "go", "mod", "edit", fmt.Sprintf("-replace=github.com/DataDog/orchestrion=%s", rootDir))
		require.NoError(t, os.WriteFile(filepath.Join(tmp, pin.OrchestrionToolGo), []byte("package tools\nimport (\n\t_ \"github.com/DataDog/orchestrion/test.circular/internal\"\n)"), 0o644))
		require.NoError(t, os.Mkdir(filepath.Join(tmp, "internal"), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(tmp, "internal", pin.OrchestrionToolGo), []byte("//go:build tools\npackage tools\nimport (\n\t_ \"github.com/DataDog/orchestrion/test.circular\"\n\t_ \"net/http\"\n)"), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(tmp, "internal", pin.OrchestrionDotYML), []byte(fmt.Sprintf("extends: [%q]", filepath.Join(rootDir, "internal", "injector", "builtin"))), 0o644))

		cfg, err := Load(tmp, false) // No validation, we've been lazy with the schema...
		require.NoError(t, err)
		require.NotNil(t, cfg)

		var yml bytes.Buffer
		enc := yaml.NewEncoder(&yml)
		require.NoError(t, enc.Encode(cfg))
		require.NoError(t, enc.Close())
		golden.AssertBytes(t, yml.Bytes(), "circular.snap.yml")

		require.Equal(t, countAspects(t, cfg), len(cfg.Aspects()))
	})

	t.Run("blank", func(t *testing.T) {
		tmp := t.TempDir()
		cfg, err := Load(tmp, true)
		require.NoError(t, err)
		require.NotNil(t, cfg)
		require.True(t, cfg.Empty())
	})

	t.Run("extends-missing-file", func(t *testing.T) {
		tmp := t.TempDir()

		require.NoError(t, os.WriteFile(filepath.Join(tmp, pin.OrchestrionDotYML), []byte(fmt.Sprintf("extends: [%q, %q]", t.TempDir(), filepath.Join(tmp, "nonexistent"))), 0o644))

		cfg, err := Load(tmp, false) // No validation, we've been lazy with the schema...
		require.ErrorContains(t, err, "no such file or directory")
		require.Nil(t, cfg)
	})
}

func countAspects(t *testing.T, cfg Config) int {
	if cfg == nil {
		return 0
	}

	switch cfg := cfg.(type) {
	case *goSource:
		total := 0
		if cfg.yaml != nil {
			total += countAspects(t, cfg.yaml)
		}
		for _, imp := range cfg.imports {
			total += countAspects(t, imp)
		}
		return total
	case *yamlSource:
		total := len(cfg.aspects)
		for _, ext := range cfg.extends {
			total += countAspects(t, ext)
		}
		return total
	default:
		require.Fail(t, "unexpected Config type: %T", cfg)
		return 0
	}
}

func run(t *testing.T, dir string, name string, args ...string) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	require.NoError(t, cmd.Run(), "running %s %s", name, args)
}

var _ yaml.Marshaler = (*goSource)(nil)

func (s *goSource) MarshalYAML() (any, error) {
	type toMarshal struct {
		Imports  []*goSource `yaml:",omitempty"`
		Declares *yamlSource `yaml:",omitempty"`
	}

	mar := map[string]toMarshal{
		s.pkgPath: {
			Imports:  s.imports,
			Declares: s.yaml,
		},
	}

	return mar, nil
}

var _ yaml.Marshaler = (*yamlSource)(nil)

func (s *yamlSource) MarshalYAML() (any, error) {
	type toMarshal struct {
		Extends []Config `yaml:",omitempty"`
		Aspects []string `yaml:",omitempty"`
	}

	aspects := make([]string, len(s.aspects))
	for i, a := range s.aspects {
		aspects[i] = a.ID
	}
	mar := map[string]toMarshal{
		clean(s.filename): {
			Extends: s.extends,
			Aspects: aspects,
		},
	}
	return mar, nil
}

func clean(filename string) string {
	noprefix := strings.TrimPrefix(filename, fmt.Sprintf("%s%c", rootDir, filepath.Separator))
	if noprefix == filename {
		return filename
	}
	return fmt.Sprintf("github.com/DataDog/orchestrion%s", noprefix)
}

var rootDir string

func init() {
	_, thisFile, _, _ := runtime.Caller(0)
	rootDir = filepath.Join(thisFile, "..", "..", "..", "..")
}
