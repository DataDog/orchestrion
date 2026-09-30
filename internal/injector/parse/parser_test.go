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

	t.Run("one candidate file", func(t *testing.T) {
		files := writeGoFiles(t, map[string]string{
			"a.go": "package example\n\n//orchestrion:enabled\nvar a bool\n",
			"b.go": large,
		})

		parsed, err := NewParser(token.NewFileSet(), len(files)).ParseFiles(context.Background(), files, aspects)
		require.NoError(t, err)
		require.Len(t, parsed, len(files))

		// All files are parsed, as type-checking needs them; but only those on which an aspect may
		// match have aspects to apply: all of them, as advice may add code that any aspect may match on.
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

func TestAnyFileMayMatch(t *testing.T) {
	aspects := []*aspect.Aspect{{ID: "enabled", JoinPoint: join.Directive("orchestrion:enabled")}}

	files := writeGoFiles(t, map[string]string{
		"a.go": "package example\n\nvar a bool\n",
		"b.go": "package example\n\n//orchestrion:enabled\nvar b bool\n",
	})
	assert.True(t, AnyFileMayMatch(files, aspects))
	assert.False(t, AnyFileMayMatch(files[:1], aspects))
	assert.False(t, AnyFileMayMatch(files, nil))

	// Large packages are parsed regardless of the aspects that may match on them.
	large := writeGoFiles(t, map[string]string{
		"a.go": "package example\n\nvar a bool\n",
		"b.go": "package example\n\nvar large = `" + strings.Repeat("x", maxBytesEagerness) + "`\n",
	})
	assert.True(t, AnyFileMayMatch(large, aspects))
	assert.True(t, AnyFileMayMatch(large, nil))

	// When the outcome cannot be determined, it is assumed an aspect may match; including when there
	// are no aspects, as ParseFiles reports malformed package clauses regardless.
	assert.True(t, AnyFileMayMatch([]string{filepath.Join(t.TempDir(), "missing.go")}, aspects))
	malformed := writeGoFiles(t, map[string]string{"c.go": "package\n"})
	assert.True(t, AnyFileMayMatch(malformed, aspects))
	assert.True(t, AnyFileMayMatch(malformed, nil))
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
