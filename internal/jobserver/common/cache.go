// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package common

import (
	"fmt"
	"sync"
	"sync/atomic"
)

type (
	Cache[V any] struct {
		mu    sync.RWMutex
		data  map[string]*slot[V]
		stats *CacheStats
	}
	slot[V any] struct {
		mu     sync.Mutex
		value  V
		loaded bool
	}

	CacheStats struct {
		total atomic.Uint64
		hits  atomic.Uint64
	}
)

func NewCache[V any](stats *CacheStats) Cache[V] {
	return Cache[V]{
		data:  make(map[string]*slot[V]),
		stats: stats,
	}
}

// Load retrieves the value for the provided key in the cache, loading it using
// the provided callback function if it is not already present. If the loader
// returns an error, the cache slot is not marked as loaded, and the error is
// returned as-is.
func (c *Cache[V]) Load(key string, loader func() (V, error)) (V, error) {
	c.stats.total.Add(1)

	s := c.slot(key)
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.loaded {
		c.stats.hits.Add(1)
		return s.value, nil
	}

	v, err := loader()
	if err != nil {
		return v, err
	}
	s.value = v
	s.loaded = true
	return v, nil
}

// slot returns the cache slot for a given key. If the slot does not exist, a
// new, un-loaded slot is allocated and stored.
func (c *Cache[V]) slot(key string) *slot[V] {
	c.mu.RLock()
	s, ok := c.data[key]
	c.mu.RUnlock()
	if ok {
		return s
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	// Check again, since we non-atomically upgraded to a write lock
	if s, ok := c.data[key]; ok {
		return s
	}
	// Allocate the new slot and return it
	s = &slot[V]{}
	c.data[key] = s
	return s
}

func (c *CacheStats) RecordHit() {
	c.total.Add(1)
	c.hits.Add(1)
}

func (c *CacheStats) RecordMiss() {
	c.total.Add(1)
}

// Hits returns the count of cache accesses that resulted in a hit.
func (c *CacheStats) Hits() uint64 {
	return c.hits.Load()
}

// Count returns the total count of cache accesses.
func (c *CacheStats) Count() uint64 {
	return c.total.Load()
}

func (c *CacheStats) String() string {
	total := c.total.Load()
	hits := c.hits.Load()

	return fmt.Sprintf("Cache hits: %d of %d (%.2f%%)",
		hits,
		total,
		100.0*float64(hits)/float64(total),
	)
}
