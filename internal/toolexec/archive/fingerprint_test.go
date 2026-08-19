// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package archive

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/blakesmith/ar"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFingerprint(t *testing.T) {
	path := filepath.Join(t.TempDir(), "subject.a")
	file, err := os.Create(path)
	require.NoError(t, err)
	_, err = file.WriteString("!<arch>\n")
	require.NoError(t, err)
	writer := ar.NewWriter(file)
	packageDefinition := []byte("exported string constant: \x00go120ldpoisoned")
	require.NoError(t, writer.WriteHeader(&ar.Header{Name: "__.PKGDEF", Mode: 0o644, Size: int64(len(packageDefinition))}))
	_, err = writer.Write(packageDefinition)
	require.NoError(t, err)

	object := []byte("go object header\n!\n\x00go120ld\x01\x23\x45\x67\x89\xab\xcd\xeftrailing data")
	require.NoError(t, writer.WriteHeader(&ar.Header{Name: "_go_.o", Mode: 0o644, Size: int64(len(object))}))
	_, err = writer.Write(object)
	require.NoError(t, err)
	require.NoError(t, file.Close())

	fingerprint, err := Fingerprint(path)
	require.NoError(t, err)
	assert.Equal(t, "0123456789abcdef", fingerprint)
}

func TestFingerprintWithLongBuildID(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "subject.go")
	require.NoError(t, os.WriteFile(source, []byte("package subject\nconst Value = 42\n"), 0o644))
	compile := func(name string, buildID string) string {
		t.Helper()
		archive := filepath.Join(dir, name)
		cmd := exec.Command("go", "tool", "compile", "-pack", "-buildid="+buildID, "-o", archive, source)
		output, err := cmd.CombinedOutput()
		require.NoError(t, err, string(output))
		return archive
	}

	shortFingerprint, err := Fingerprint(compile("short.a", "short"))
	require.NoError(t, err)
	longFingerprint, err := Fingerprint(compile("long.a", strings.Repeat("x", 5_000)))
	require.NoError(t, err)
	assert.Equal(t, shortFingerprint, longFingerprint)
	assert.NotEqual(t, strings.Repeat("0", fingerprintSize*2), longFingerprint)
}

func TestFingerprintErrors(t *testing.T) {
	fingerprint, err := Fingerprint(filepath.Join(t.TempDir(), "missing.a"))
	assert.Empty(t, fingerprint)
	require.ErrorContains(t, err, "opening Go archive")

	path := writeArchiveEntry(t, "__.PKGDEF", []byte("package definition"))
	fingerprint, err = Fingerprint(path)
	assert.Empty(t, fingerprint)
	require.ErrorIs(t, err, ErrNoFingerprint)

	fingerprint, err = CompatibilityFingerprint(path)
	require.NoError(t, err)
	assert.Equal(t, "none", fingerprint)
}

func TestFingerprintRejectsMalformedGoObject(t *testing.T) {
	tests := []struct {
		name   string
		object []byte
	}{
		{name: "missing header terminator", object: []byte("go object header")},
		{name: "truncated binary header", object: []byte("go object header\n!\n\x00go120ld\x01\x23")},
		{name: "unexpected object magic", object: []byte("go object header\n!\nnotmagic\x01\x23\x45\x67\x89\xab\xcd\xef")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeArchiveEntry(t, "_go_.o", tt.object)
			_, err := Fingerprint(path)
			require.ErrorIs(t, err, errMalformedGoObject)
			_, err = CompatibilityFingerprint(path)
			require.ErrorIs(t, err, errMalformedGoObject)
		})
	}
}

func TestFingerprintDoesNotScanFollowingArchiveMembers(t *testing.T) {
	path := filepath.Join(t.TempDir(), "subject.a")
	file, err := os.Create(path)
	require.NoError(t, err)
	writer := ar.NewWriter(file)
	require.NoError(t, writer.WriteGlobalHeader())
	first := []byte("go object header without terminator")
	require.NoError(t, writer.WriteHeader(&ar.Header{Name: "_go_.o", Mode: 0o644, Size: int64(len(first))}))
	_, err = writer.Write(first)
	require.NoError(t, err)
	second := []byte("\n!\n\x00go120ld\x01\x23\x45\x67\x89\xab\xcd\xef")
	require.NoError(t, writer.WriteHeader(&ar.Header{Name: "second.o", Mode: 0o644, Size: int64(len(second))}))
	_, err = writer.Write(second)
	require.NoError(t, err)
	require.NoError(t, file.Close())

	_, err = Fingerprint(path)
	require.ErrorIs(t, err, errMalformedGoObject)
}

func writeArchiveEntry(t *testing.T, name string, contents []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "subject.a")
	file, err := os.Create(path)
	require.NoError(t, err)
	writer := ar.NewWriter(file)
	require.NoError(t, writer.WriteGlobalHeader())
	require.NoError(t, writer.WriteHeader(&ar.Header{Name: name, Mode: 0o644, Size: int64(len(contents))}))
	_, err = writer.Write(contents)
	require.NoError(t, err)
	require.NoError(t, file.Close())
	return path
}
