// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

// Package archive reads metadata from Go object archives.
package archive

import (
	"bufio"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/blakesmith/ar"
)

// ErrNoFingerprint reports an archive without compiler export-data identity.
var ErrNoFingerprint = errors.New("Go archive has no object fingerprint")

var errMalformedGoObject = errors.New("malformed Go object header")

const (
	goObjectHeaderEnd = "\n!\n"
	goObjectMagic     = "\x00go120ld"
	fingerprintSize   = 8
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
		fingerprint, err := readObjectFingerprint(io.LimitReader(rd, hdr.Size))
		if err != nil {
			return "", fmt.Errorf("reading Go object %q in %q: %w", hdr.Name, filename, err)
		}
		return fingerprint, nil
	}
}

// readObjectFingerprint scans the variable-length textual header of a _go_.o
// linker object, which ends in "\n!\n", then reads the binary object magic and
// fingerprint that immediately follow it.
func readObjectFingerprint(r io.Reader) (string, error) {
	rd := bufio.NewReader(r)
	var first, second, third byte
	for {
		value, err := rd.ReadByte()
		if errors.Is(err, io.EOF) {
			return "", fmt.Errorf("%w: missing textual header terminator", errMalformedGoObject)
		}
		if err != nil {
			return "", err
		}
		first, second, third = second, third, value
		if first == goObjectHeaderEnd[0] && second == goObjectHeaderEnd[1] && third == goObjectHeaderEnd[2] {
			break
		}
	}

	var header [len(goObjectMagic) + fingerprintSize]byte
	if _, err := io.ReadFull(rd, header[:]); err != nil {
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			return "", fmt.Errorf("%w: truncated binary header", errMalformedGoObject)
		}
		return "", err
	}
	if string(header[:len(goObjectMagic)]) != goObjectMagic {
		return "", fmt.Errorf("%w: unexpected object magic", errMalformedGoObject)
	}
	return hex.EncodeToString(header[len(goObjectMagic):]), nil
}
