// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package fingerprint

import (
	"crypto/sha512"
	"encoding/base64"
	"hash"
	"io"
	"strconv"
	"sync"
)

type Hasher struct {
	hash hash.Hash
}

type Hashable interface {
	Hash(h *Hasher) error
}

var pool = sync.Pool{New: func() any { return &Hasher{hash: sha512.New()} }}

// New returns a [Hasher] from the pool, ready to use.
func New() *Hasher {
	h, _ := pool.Get().(*Hasher)
	return h
}

// Close returns this [Hasher] to the pool.
func (h *Hasher) Close() {
	h.hash.Reset()
	pool.Put(h)
}

// Finish obtains this [Hasher]'s current fingerprint. It does not change the
// underlying state of the [Hasher].
func (h *Hasher) Finish() string {
	var buf [sha512.Size]byte
	return base64.URLEncoding.EncodeToString(h.hash.Sum(buf[:0]))
}

// Named hashes a named list of values. This creates explicit grouping of the
// values, avoiding that the concatenation of two things has a different hash
// than those same two things one after the other.
func (h *Hasher) Named(name string, vals ...Hashable) error {
	var (
		soh = []byte{1} // Start of key-value-pair beacon
		sot = []byte{2} // Start of value & end of key beacon
		etx = []byte{3} // End of key-value-pair beacon
	)

	if _, err := h.hash.Write(soh); err != nil {
		return err
	}

	if _, err := io.WriteString(h.hash, name); err != nil {
		return err
	}

	if _, err := h.hash.Write(sot); err != nil {
		return err
	}

	for idx, val := range vals {
		if _, err := h.hash.Write(soh); err != nil {
			return err
		}
		if _, err := io.WriteString(h.hash, strconv.Itoa(idx)); err != nil {
			return err
		}
		if _, err := h.hash.Write(sot); err != nil {
			return err
		}
		if err := val.Hash(h); err != nil {
			return err
		}
		if _, err := h.hash.Write(etx); err != nil {
			return err
		}
	}

	if _, err := h.hash.Write(etx); err != nil {
		return err
	}

	_, err := io.WriteString(h.hash, name)
	return err
}

// Fingerprint is a short-hand for creating a new [Hasher], calling
// [Hashable.Hash] on the provided value (unless it is nil), and then returning
// the [Hasher.Finish] result.
func Fingerprint(val Hashable) (string, error) {
	h := New()
	defer h.Close()

	if val != nil {
		if err := val.Hash(h); err != nil {
			return "", err
		}
	}

	return h.Finish(), nil
}
