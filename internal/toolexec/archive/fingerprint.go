// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

// Package archive reads metadata from Go object archives.
package archive

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/blakesmith/ar"
)

// ErrNoFingerprint reports an archive without compiler export-data identity.
var ErrNoFingerprint = errors.New("Go archive has no object fingerprint")

const (
	goObjectMagic      = "\x00go120ld"
	fingerprintSize    = 8
	objectHeaderPrefix = 4 << 10
)

// CompatibilityFingerprint returns Fingerprint, or "none" for an archive
// without compiler export data and therefore without a linker compatibility identity.
func CompatibilityFingerprint(filename string) (string, error) {
	fingerprint, err := Fingerprint(filename)
	if errors.Is(err, ErrNoFingerprint) {
		return "none", nil
	}
	return fingerprint, err
}

// Fingerprint returns the compiler export-data fingerprint stored in a Go
// archive. Go uses this value to validate every compiler-import edge at link
// time, so equal fingerprints are the compatibility identity needed by test
// variant reconstruction.
func Fingerprint(filename string) (string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return "", fmt.Errorf("opening Go archive %q: %w", filename, err)
	}
	defer file.Close()

	rd := ar.NewReader(file)
	for {
		hdr, err := rd.Next()
		if errors.Is(err, io.EOF) {
			return "", fmt.Errorf("%w: %q", ErrNoFingerprint, filename)
		}
		if err != nil {
			return "", fmt.Errorf("reading Go archive %q: %w", filename, err)
		}
		if hdr.Name != "_go_.o" {
			continue
		}
		data, err := io.ReadAll(io.LimitReader(rd, objectHeaderPrefix))
		if err != nil {
			return "", fmt.Errorf("reading Go object %q in %q: %w", hdr.Name, filename, err)
		}
		index := bytes.Index(data, []byte(goObjectMagic))
		if index < 0 || len(data) < index+len(goObjectMagic)+fingerprintSize {
			continue
		}
		fingerprint := data[index+len(goObjectMagic) : index+len(goObjectMagic)+fingerprintSize]
		return hex.EncodeToString(fingerprint), nil
	}
}
