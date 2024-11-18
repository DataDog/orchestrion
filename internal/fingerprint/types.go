// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package fingerprint

import (
	"io"
	"slices"
	"strconv"
	"strings"
)

type Bool bool

func (b Bool) Hash(h *Hasher) error {
	_, err := io.WriteString(h.hash, strconv.FormatBool(bool(b)))
	return err
}

type Int int

func (i Int) Hash(h *Hasher) error {
	_, err := io.WriteString(h.hash, strconv.Itoa(int(i)))
	return err
}

type List[T Hashable] []T

func (l List[T]) Hash(h *Hasher) error {
	list := make([]Hashable, len(l)+1)
	list[0] = Int(len(l))
	for idx, val := range l {
		list[idx+1] = val
	}

	return h.Named("list", list...)
}

type String string

func (s String) Hash(h *Hasher) error {
	_, err := io.WriteString(h.hash, string(s))
	return err
}

func Cast[E any, T ~[]E, H Hashable](slice T, fn func(E) H) List[H] {
	res := make(List[H], len(slice))
	for idx, val := range slice {
		res[idx] = fn(val)
	}
	return res
}

type (
	mapped[T Hashable]     []mappedItem[T]
	mappedItem[T Hashable] struct {
		key string
		val T
	}
)

func Map[K comparable, V any, H Hashable](m map[K]V, fn func(K, V) (string, H)) Hashable {
	res := make(mapped[H], 0, len(m))

	for key, val := range m {
		mkey, mval := fn(key, val)
		res = append(res, mappedItem[H]{mkey, mval})
	}

	slices.SortFunc(res, func(l mappedItem[H], r mappedItem[H]) int {
		return strings.Compare(l.key, r.key)
	})

	return res
}

func (m mapped[T]) Hash(h *Hasher) error {
	list := make([]Hashable, len(m)+1)
	list[0] = Int(len(m))
	for idx, item := range m {
		list[idx+1] = item
	}

	return h.Named("map", list...)
}

func (m mappedItem[T]) Hash(h *Hasher) error {
	return h.Named(m.key, m.val)
}
