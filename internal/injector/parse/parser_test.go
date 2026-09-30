// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package parse

import (
	"context"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/DataDog/orchestrion/internal/injector/aspect"
	"github.com/DataDog/orchestrion/internal/injector/aspect/join"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseFiles(t *testing.T) {
	enabled := &aspect.Aspect{ID: "enabled", JoinPoint: join.Directive("orchestrion:enabled")}
	version := &aspect.Aspect{ID: "version", JoinPoint: join.Directive("orchestrion:version")}
	aspects := []*aspect.Aspect{enabled, version}

	// Large enough that the package exceeds maxBytesEagerness.
	large := "package example\n\nvar large = `" + strings.Repeat("x", maxBytesEagerness) + "`\n"

	t.Run("no candidate file", func(t *testing.T) {
		files := writeGoFiles(t, map[string]string{
			"a.go": "package example\n\nvar a bool\n",
			"b.go": "package example\n\nvar b bool\n",
		})

		parsed, err := NewParser(token.NewFileSet(), len(files)).ParseFiles(context.Background(), files, aspects)
		require.NoError(t, err)
		assert.Empty(t, parsed)
	})

	t.Run("no candidate file in a large package", func(t *testing.T) {
		files := writeGoFiles(t, map[string]string{
			"a.go": "package example\n\nvar a bool\n",
			"b.go": large,
		})

		parsed, err := NewParser(token.NewFileSet(), len(files)).ParseFiles(context.Background(), files, aspects)
		require.NoError(t, err)
		require.Len(t, parsed, len(files))

		// All files are parsed, but none has any aspect that may match on it.
		for _, file := range parsed {
			require.NotNil(t, file.AstFile, "file %q was not parsed", file.Name)
			assert.Empty(t, file.Aspects, "file %q", file.Name)
		}
	})

	t.Run("one candidate file in a large package", func(t *testing.T) {
		files := writeGoFiles(t, map[string]string{
			"a.go": "package example\n\n//orchestrion:enabled\nvar a bool\n",
			"b.go": large,
		})

		parsed, err := NewParser(token.NewFileSet(), len(files)).ParseFiles(context.Background(), files, aspects)
		require.NoError(t, err)
		require.Len(t, parsed, len(files))

		// All files are parsed (type-checking needs them), but only a.go gets aspects: all of them.
		for _, file := range parsed {
			require.NotNil(t, file.AstFile, "file %q was not parsed", file.Name)
			switch filepath.Base(file.Name) {
			case "a.go":
				assert.Equal(t, aspects, file.Aspects)
			case "b.go":
				assert.Empty(t, file.Aspects)
			}
		}
	})
}

// writeGoFiles writes the provided files in a temporary directory, and returns their paths in lexical
// order of their names.
func writeGoFiles(t *testing.T, files map[string]string) []string {
	t.Helper()

	dir := t.TempDir()
	paths := make([]string, 0, len(files))
	for name, content := range files {
		path := filepath.Join(dir, name)
		require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
		paths = append(paths, path)
	}
	slices.Sort(paths)
	return paths
}
