// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package aspect

import (
	"context"
	"runtime"
	"sync"
)

// resolutionPrefetcher starts package resolutions ahead of the (sequential) code that needs their
// results. The job server caches successful resolutions, and makes identical requests wait for the one
// in progress; so when the sequential code requests a prefetched resolution, it receives the same
// response it would have received without prefetching, only sooner. Prefetching is best-effort: the
// prefetched results and errors are discarded. Failed resolutions are not cached, so the sequential
// code performs them again when it makes the same request, and reports their error as before.
type resolutionPrefetcher struct {
	ctx     context.Context
	closed  chan struct{} // Closed by [resolutionPrefetcher.Close]
	init    func() error
	resolve func(ctx context.Context, importPath string) error

	initialized bool
	disabled    bool
	started     map[string]struct{}
	sem         chan struct{}
	wg          sync.WaitGroup
}

// newResolutionPrefetcher creates a new [resolutionPrefetcher] that uses resolve to resolve import
// paths. The init function is called (once) before the first resolution is started, and prefetching
// is disabled if it returns an error. The returned prefetcher must be closed once no longer needed.
func newResolutionPrefetcher(ctx context.Context, init func() error, resolve func(ctx context.Context, importPath string) error) *resolutionPrefetcher {
	return &resolutionPrefetcher{
		ctx:     ctx,
		closed:  make(chan struct{}),
		init:    init,
		resolve: resolve,
		started: make(map[string]struct{}),
		sem:     make(chan struct{}, runtime.GOMAXPROCS(0)),
	}
}

// Prefetch starts resolving importPath in the background, unless it was already started. It must not
// be called concurrently.
func (p *resolutionPrefetcher) Prefetch(importPath string) {
	if !p.initialized {
		p.initialized = true
		p.disabled = p.init() != nil
	}
	if p.disabled {
		return
	}
	if _, started := p.started[importPath]; started {
		return
	}
	p.started[importPath] = struct{}{}

	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		select {
		case p.sem <- struct{}{}:
			defer func() { <-p.sem }()
		case <-p.closed:
			return
		}
		select {
		case <-p.closed:
			// Both channels were ready, and the semaphore was selected.
			return
		default:
		}
		_ = p.resolve(p.ctx, importPath)
	}()
}

// Close prevents the prefetches that have not started yet from starting, and waits for the others to
// complete. These are not cancelled, as the job server would perform them regardless: the nested
// builds they involve must not outlive this process, as the go command removes the directory they
// work in once it completes.
func (p *resolutionPrefetcher) Close() {
	close(p.closed)
	p.wg.Wait()
}
