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
// copy-on-write pointer: [newSlot] never mutates the slice a concurrent
// reader might be holding, it builds a new one and swaps the pointer, so
// [registrySnapshot] can never observe a data race against a concurrent
// [newSlot] call.
//
// In practice, [Register] (and therefore [newSlot]) is documented to only
// ever be called at init time, and Go guarantees init (across every
// package) completes -- with a full happens-before edge -- before main()
// starts and before any goroutine can be spawned. So by the time
// [registrySnapshot] is ever read concurrently (from WrapGoroutine,
// Bootstrap, or a [Chan]), the registry is already expected to be
// immutable in practice. This atomic pointer closes the theoretical race
// that would otherwise exist if a [Register] call happened later than
// documented, at near-zero cost to readers.
//
// This uses "sync/atomic" rather than "sync" (e.g. a [sync.RWMutex])
// because [WrapGoroutine] is woven into every `go` statement orchestrion
// compiles, including ones inside sync's own transitive dependencies --
// importing "sync" here would risk introducing an import cycle invisible
// to Go's static build graph (it only appears once toolexec starts
// rewriting `go` statements at compile time). "sync/atomic"'s entire
// transitive dependency closure is just "unsafe", neither of which
// contains a `go` statement for orchestrion to weave, so it carries none
// of that risk.
var registry atomic.Pointer[[]entry]

// newSlot registers hooks under a freshly allocated index and returns the
// [slot] handle for it. Still expected to only be called at init time (see
// [Register]), but the copy-on-write update below makes any concurrent
// [registrySnapshot] read race-free regardless.
func newSlot[T any](hooks Hooks[T]) *slot[T] {
	old := registry.Load()
	var current []entry
	if old != nil {
		current = *old
	}

	index := len(current)
	updated := make([]entry, index, index+1)
	copy(updated, current)
	updated = append(updated, typedEntry[T]{hooks: hooks})

	registry.Store(&updated)
	return &slot[T]{index: index}
}

// registrySnapshot returns the current set of registered entries.
func registrySnapshot() []entry {
	current := registry.Load()
	if current == nil {
		return nil
	}
	return *current
}
