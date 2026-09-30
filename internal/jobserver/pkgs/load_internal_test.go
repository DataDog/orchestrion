// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package pkgs

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/DataDog/orchestrion/internal/jobserver/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/packages"
)

func TestLoadReportsInPatternOrder(t *testing.T) {
	files := map[string]string{
		"go.mod":      "module example.com/load\n\ngo 1.25\n",
		"a/a.go":      "package a\n",
		"b/b.go":      "package b\n",
		"c/c.go":      "package c\n",
		"fast/x/x.go": "package x\n",
		"fast/y/y.go": "package y\n",
	}
	for i := range 100 { // Listing "./slow/..." takes longer than listing "./fast/..."
		files[fmt.Sprintf("slow/p%d/p.go", i)] = "package p\n"
	}
	dir := t.TempDir()
	for name, content := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	}

	ctx := context.Background()
	s := &service{loaded: common.NewCache[*packages.Package](&common.CacheStats{})}

	t.Run("results", func(t *testing.T) {
		resp, err := s.load(ctx, LoadRequest{Dir: dir, Patterns: []string{"./c", "./a", "./b"}})
		require.NoError(t, err)
		require.Len(t, resp, 3)
		assert.Equal(t, "example.com/load/c", resp[0].PkgPath)
		assert.Equal(t, "example.com/load/a", resp[1].PkgPath)
		assert.Equal(t, "example.com/load/b", resp[2].PkgPath)
	})

	t.Run("errors", func(t *testing.T) {
		// Both multi-package patterns fail to load; the first one in pattern order must be reported, even
		// though it usually fails last.
		_, err := s.load(ctx, LoadRequest{Dir: dir, Patterns: []string{"./a", "./slow/...", "./fast/..."}})
		require.ErrorContains(t, err, `loading "./slow/...": expected 1 package for "./slow/...", got 100`)
	})
}
