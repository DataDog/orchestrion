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

	"github.com/blakesmith/ar"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTestMainMetadata(t *testing.T) {
	archive := filepath.Join(t.TempDir(), "main.a")
	require.NoError(t, os.WriteFile(archive, []byte("!<arch>\n"), 0o644))
	file, err := os.OpenFile(archive, os.O_APPEND|os.O_WRONLY, 0o644)
	require.NoError(t, err)
	contents := []byte("\x00go120ld\x01\x23\x45\x67\x89\xab\xcd\xef")
	writer := ar.NewWriter(file)
	require.NoError(t, writer.WriteHeader(&ar.Header{Name: "__.PKGDEF", Mode: 0o644, Size: int64(len(contents))}))
	_, err = writer.Write(contents)
	require.NoError(t, err)
	require.NoError(t, file.Close())

	compile := &CompileCommand{Flags: compileFlagSet{Output: archive}}
	compile.MarkTestMain("example.com/subject")
	compile.SetTestMainPackageFiles(map[string]string{"example.com/importer": archive})
	compile.AddTestMainReverseRoot("example.com/importer")
	require.NoError(t, compile.attachTestMain())

	link := &LinkCommand{Inputs: []string{archive}}
	info, ok, err := link.TestMainInfo(context.Background())
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, TestMainInfo{
		Target:              "example.com/subject",
		PackageFiles:        map[string]string{"example.com/importer": archive},
		PackageFingerprints: map[string]string{"example.com/importer": "0123456789abcdef"},
		ReverseRoots:        []string{"example.com/importer"},
	}, info)

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
