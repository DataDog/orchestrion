// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package context

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

func (e typedEntry[T]) chanRecv(parent, sent any) any {
	p, _ := parent.(*Stack[T])
	s, _ := sent.(*Stack[T])
	return e.hooks.ChanRecv(p, s)
}

// registry is deliberately unsynchronized: [Register] is documented to only
// ever be called at init time, and Go guarantees init (across every package)
// completes -- with a full happens-before edge -- before main() starts and
// before any goroutine can be spawned. So by the time [registrySnapshot] is
// ever read concurrently (from WrapGoroutine, Bootstrap, or a [Chan]), the
// slice is already immutable. This lets this package avoid importing "sync"
// entirely, which matters because [WrapGoroutine] is woven into every `go`
// statement orchestrion compiles, including ones inside sync's own
// transitive dependencies -- importing sync here would risk introducing an
// import cycle invisible to Go's static build graph (it only appears once
// toolexec starts rewriting `go` statements at compile time).
var registry []entry

// newSlot registers hooks under a freshly allocated index and returns the
// [slot] handle for it.
func newSlot[T any](hooks Hooks[T]) *slot[T] {
	index := len(registry)
	registry = append(registry, typedEntry[T]{hooks: hooks})
	return &slot[T]{index: index}
}

// registrySnapshot returns the current set of registered entries.
func registrySnapshot() []entry {
	return registry
}
