// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package context

import "sync/atomic"

// slot is a single [Register] call's handle into the per-goroutine context
// blob: a fixed index assigned at registration time.
type slot[T any] struct {
	index int
}

// current returns the current goroutine's [Stack] for T, creating (and
// installing) an empty one if none exists yet.
func (s *slot[T]) current() *Stack[T] {
	if st, ok := getBlobEntry(s.index).(*Stack[T]); ok && st != nil {
		return st
	}
	st := new(Stack[T])
	setBlobEntry(s.index, st)
	return st
}

// entry is the type-erased view of a single [Register] call, used to
// dispatch Main/Go/ChanSend/ChanRecv across every registered type at once,
// regardless of what T each one was registered with.
type entry interface {
	main() any
	goHook(parent any) any
	chanSend(parent any) any
	chanRecv(parent, sent any) any
}

type typedEntry[T any] struct {
	hooks Hooks[T]
}

func (e typedEntry[T]) main() any { return e.hooks.Main() }

func (e typedEntry[T]) goHook(parent any) any {
	p, _ := parent.(*Stack[T])
	return e.hooks.Go(p)
}

func (e typedEntry[T]) chanSend(parent any) any {
	p, _ := parent.(*Stack[T])
	return e.hooks.ChanSend(p)
}

func (e typedEntry[T]) chanRecv(parent any, sent any) any {
	p, _ := parent.(*Stack[T])
	s, _ := sent.(*Stack[T])
	return e.hooks.ChanRecv(p, s)
}

// registry holds the current set of registered entries as an atomic
// copy-on-write pointer. newSlot uses a compare-and-swap loop, so concurrent
// registrations cannot overwrite each other and readers always observe a
// complete immutable snapshot. This matters during initialization: init
// functions may start goroutines before later packages finish registering.
//
// This uses "sync/atomic" rather than "sync" to keep the package's dependency
// closure minimal; runtime imports this package's behavior only through a
// reverse callback and must never acquire an application-level lock.
var registry atomic.Pointer[[]entry]

// newSlot registers hooks under a freshly allocated index and returns the
// [slot] handle for it. Still expected to only be called at init time (see
// [Register]), but the copy-on-write update below makes any concurrent
// [registrySnapshot] read race-free regardless.
func newSlot[T any](hooks Hooks[T]) *slot[T] {
	for {
		old := registry.Load()
		var current []entry
		if old != nil {
			current = *old
		}

		index := len(current)
		updated := make([]entry, index, index+1)
		copy(updated, current)
		updated = append(updated, typedEntry[T]{hooks: hooks})

		if registry.CompareAndSwap(old, &updated) {
			return &slot[T]{index: index}
		}
	}
}

// registrySnapshot returns the current set of registered entries.
func registrySnapshot() []entry {
	current := registry.Load()
	if current == nil {
		return nil
	}
	return *current
}
