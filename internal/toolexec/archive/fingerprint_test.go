// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package archive

import (
	"os"
	"path/filepath"
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

	object := []byte("go object header\n\x00go120ld\x01\x23\x45\x67\x89\xab\xcd\xeftrailing data")
	require.NoError(t, writer.WriteHeader(&ar.Header{Name: "_go_.o", Mode: 0o644, Size: int64(len(object))}))
	_, err = writer.Write(object)
	require.NoError(t, err)
	require.NoError(t, file.Close())

	fingerprint, err := Fingerprint(path)
	require.NoError(t, err)
	assert.Equal(t, "0123456789abcdef", fingerprint)
}

func TestFingerprintErrors(t *testing.T) {
	fingerprint, err := Fingerprint(filepath.Join(t.TempDir(), "missing.a"))
	assert.Empty(t, fingerprint)
	require.ErrorContains(t, err, "opening Go archive")

	path := filepath.Join(t.TempDir(), "subject.a")
	file, err := os.Create(path)
	require.NoError(t, err)
	writer := ar.NewWriter(file)
	require.NoError(t, writer.WriteGlobalHeader())
	object := []byte("go object header\n\x00go120ld\x01\x23")
	require.NoError(t, writer.WriteHeader(&ar.Header{Name: "_go_.o", Mode: 0o644, Size: int64(len(object))}))
	_, err = writer.Write(object)
	require.NoError(t, err)
	require.NoError(t, file.Close())

	fingerprint, err = Fingerprint(path)
	assert.Empty(t, fingerprint)
	require.ErrorIs(t, err, ErrNoFingerprint)

	fingerprint, err = CompatibilityFingerprint(path)
	require.NoError(t, err)
	assert.Equal(t, "none", fingerprint)
}
