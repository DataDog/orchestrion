// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package proxy

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTestMainMetadata(t *testing.T) {
	archive := filepath.Join(t.TempDir(), "main.a")
	require.NoError(t, os.WriteFile(archive, []byte("!<arch>\n"), 0o644))

	compile := &CompileCommand{Flags: compileFlagSet{Output: archive}}
	compile.MarkTestMain("example.com/subject")
	require.NoError(t, compile.attachTestMain())

	link := &LinkCommand{Inputs: []string{archive}}
	target, ok, err := link.TestVariantFor(context.Background())
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, "example.com/subject", target)
}

func TestTestMainMetadataIgnoresImportPathSuffix(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "ordinary.test.a")
	require.NoError(t, os.WriteFile(archive, []byte("!<arch>\n"), 0o644))
	nonArchive := filepath.Join(dir, "external.o")
	require.NoError(t, os.WriteFile(nonArchive, []byte("object"), 0o644))

	link := &LinkCommand{Inputs: []string{archive, nonArchive}}
	target, ok, err := link.TestVariantFor(context.Background())
	require.NoError(t, err)
	assert.False(t, ok)
	assert.Empty(t, target)
}
