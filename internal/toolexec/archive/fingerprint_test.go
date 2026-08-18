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
	contents := []byte("go object header\n\x00go120ld\x01\x23\x45\x67\x89\xab\xcd\xeftrailing data")
	require.NoError(t, writer.WriteHeader(&ar.Header{Name: "__.PKGDEF", Mode: 0o644, Size: int64(len(contents))}))
	_, err = writer.Write(contents)
	require.NoError(t, err)
	require.NoError(t, file.Close())

	fingerprint, err := Fingerprint(path)
	require.NoError(t, err)
	assert.Equal(t, "0123456789abcdef", fingerprint)
}
