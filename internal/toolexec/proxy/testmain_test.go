// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package proxy

import (
	"context"
	"encoding/json"
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
	contents := []byte("go object header\n!\n\x00go120ld\x01\x23\x45\x67\x89\xab\xcd\xef")
	writer := ar.NewWriter(file)
	require.NoError(t, writer.WriteHeader(&ar.Header{Name: "_go_.o", Mode: 0o644, Size: int64(len(contents))}))
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

func TestTestMainMetadataV1(t *testing.T) {
	archive := writeTestMainMetadataArchive(t, testMainHeaderV1+"\nexample.com/subject\n")
	info, ok, err := (&LinkCommand{Inputs: []string{archive}}).TestMainInfo(context.Background())
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, TestMainInfo{Target: "example.com/subject"}, info)
}

func TestTestMainMetadataConflicts(t *testing.T) {
	t.Run("target", func(t *testing.T) {
		first := writeTestMainMetadataArchive(t, testMainHeaderV1+"\nexample.com/first\n")
		second := writeTestMainMetadataArchive(t, testMainHeaderV1+"\nexample.com/second\n")
		_, _, err := (&LinkCommand{Inputs: []string{first, second}}).TestMainInfo(context.Background())
		require.ErrorContains(t, err, "conflicting test-main targets")
	})

	t.Run("metadata", func(t *testing.T) {
		first := writeTestMainInfoArchive(t, TestMainInfo{
			Target:       "example.com/subject",
			PackageFiles: map[string]string{"example.com/root": "/first.a"},
		})
		second := writeTestMainInfoArchive(t, TestMainInfo{
			Target:       "example.com/subject",
			PackageFiles: map[string]string{"example.com/root": "/second.a"},
		})
		_, _, err := (&LinkCommand{Inputs: []string{first, second}}).TestMainInfo(context.Background())
		require.ErrorContains(t, err, "conflicting test-main metadata")
	})
}

func TestReadTestMainRejectsMalformedMetadata(t *testing.T) {
	tests := []struct {
		name     string
		contents string
		want     string
	}{
		{name: "empty", want: "invalid test-main metadata"},
		{name: "missing target", contents: testMainHeaderV1 + "\n", want: "missing test-main target"},
		{name: "invalid JSON", contents: testMainHeaderV2 + "\n{\n", want: "invalid test-main metadata"},
		{name: "unknown header", contents: "#unknown\nexample.com/subject\n", want: "invalid test-main metadata"},
		{name: "trailing data", contents: testMainHeaderV1 + "\nexample.com/subject\nextra\n", want: "invalid test-main metadata"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := readTestMain(writeTestMainMetadataArchive(t, tt.contents))
			require.ErrorContains(t, err, tt.want)
		})
	}
}

func TestAttachTestMainRejectsMissingReverseArchive(t *testing.T) {
	archive := filepath.Join(t.TempDir(), "main.a")
	require.NoError(t, os.WriteFile(archive, []byte("!<arch>\n"), 0o644))
	compile := &CompileCommand{Flags: compileFlagSet{Output: archive}}
	compile.MarkTestMain("example.com/subject")
	compile.SetTestMainPackageFiles(map[string]string{"example.com/importer": filepath.Join(t.TempDir(), "missing.a")})
	require.ErrorContains(t, compile.attachTestMain(), "fingerprinting reverse test variant")
}

func writeTestMainInfoArchive(t *testing.T, info TestMainInfo) string {
	t.Helper()
	payload, err := json.Marshal(info)
	require.NoError(t, err)
	return writeTestMainMetadataArchive(t, testMainHeaderV2+"\n"+string(payload)+"\n")
}

func writeTestMainMetadataArchive(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "main.a")
	file, err := os.Create(path)
	require.NoError(t, err)
	writer := ar.NewWriter(file)
	require.NoError(t, writer.WriteGlobalHeader())
	require.NoError(t, writer.WriteHeader(&ar.Header{Name: testMainFilename, Mode: 0o644, Size: int64(len(contents))}))
	_, err = writer.Write([]byte(contents))
	require.NoError(t, err)
	require.NoError(t, file.Close())
	return path
}
