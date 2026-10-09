// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package aspect

import (
	"context"
	"errors"
	"runtime"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolutionPrefetcher(t *testing.T) {
	t.Run("resolves each import path once", func(t *testing.T) {
		var (
			mu       sync.Mutex
			resolved []string
			inits    int
		)
		p := newResolutionPrefetcher(context.Background(),
			func() error { inits++; return nil },
			func(_ context.Context, importPath string) error {
				mu.Lock()
				defer mu.Unlock()
				resolved = append(resolved, importPath)
				return errors.New("errors are ignored")
			},
		)
		for _, importPath := range []string{"a", "b", "a", "c", "b"} {
			p.Prefetch(importPath)
		}
		require.Eventually(t, func() bool {
			mu.Lock()
			defer mu.Unlock()
			return len(resolved) == 3
		}, 10*time.Second, time.Millisecond)
		p.Close()

		assert.Equal(t, 1, inits)
		slices.Sort(resolved)
		assert.Equal(t, []string{"a", "b", "c"}, resolved)
	})

	t.Run("disabled if initialization fails", func(t *testing.T) {
		var inits, resolutions atomic.Int32
		p := newResolutionPrefetcher(context.Background(),
			func() error { inits.Add(1); return errors.New("no job server") },
			func(context.Context, string) error { resolutions.Add(1); return nil },
		)
		p.Prefetch("a")
		p.Prefetch("b")
		p.Close()

		assert.EqualValues(t, 1, inits.Load())
		assert.Zero(t, resolutions.Load())
	})

	t.Run("bounds concurrency, and close only waits for running resolutions", func(t *testing.T) {
		var running, maxRunning, started atomic.Int32
		release := make(chan struct{})
		p := newResolutionPrefetcher(context.Background(),
			func() error { return nil },
			func(context.Context, string) error {
				started.Add(1)
				n := running.Add(1)
				defer running.Add(-1)
				for {
					if m := maxRunning.Load(); n <= m || maxRunning.CompareAndSwap(m, n) {
						break
					}
				}
				<-release
				return nil
			},
		)
		limit := runtime.GOMAXPROCS(0)
		for i := range 3 * limit {
			p.Prefetch(string(rune('a' + i)))
		}
		require.Eventually(t, func() bool { return running.Load() == int32(limit) }, 10*time.Second, time.Millisecond)

		closed := make(chan struct{})
		go func() {
			defer close(closed)
			p.Close()
		}()
		<-p.closed

		// Close waits for the running resolutions to complete...
		select {
		case <-closed:
			require.Fail(t, "Close returned while resolutions were still running")
		case <-time.After(50 * time.Millisecond):
		}
		close(release)
		select {
		case <-closed:
		case <-time.After(10 * time.Second):
			require.Fail(t, "Close did not return once resolutions completed")
		}

		// ... but the pending ones never start.
		assert.EqualValues(t, limit, maxRunning.Load())
		assert.EqualValues(t, limit, started.Load())
	})
}
