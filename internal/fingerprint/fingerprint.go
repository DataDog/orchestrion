// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package fingerprint

import (
	"crypto/sha512"
	"encoding/base64"
	"fmt"
	"hash"
	"sync"
)

type Hasher struct {
	hash hash.Hash
}

type Hashable interface {
	Hash(h *Hasher) error
}

var pool = sync.Pool{New: func() any { return &Hasher{hash: sha512.New()} }}

func Fingerprint(val Hashable) (string, error) {
	h, _ := pool.Get().(*Hasher)
	defer func() {
		h.hash.Reset()
		pool.Put(h)
	}()

	if val != nil {
		if err := val.Hash(h); err != nil {
			return "", err
		}
	}

	var buf [sha512.Size]byte
	return base64.URLEncoding.EncodeToString(h.hash.Sum(buf[:0])), nil
}

func (h *Hasher) Named(name string, vals ...Hashable) error {
	if _, err := fmt.Fprintf(h.hash, "\x01%s\x02", name); err != nil {
		return err
	}

	for idx, val := range vals {
		if _, err := fmt.Fprintf(h.hash, "\x01%d\x02", idx); err != nil {
			return err
		}
		if err := val.Hash(h); err != nil {
			return err
		}
		if _, err := fmt.Fprint(h.hash, "x03"); err != nil {
			return err
		}
	}

	_, err := fmt.Fprint(h.hash, "\x03", name)
	return err
}
