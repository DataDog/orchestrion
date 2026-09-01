// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package context

// WrapGoroutine is woven by orchestrion into every `go` statement it
// compiles, turning `go f(args...)` into `go WrapGoroutine(func() {
// f(args...) })()`. It captures the calling goroutine's context stacks
// synchronously (since WrapGoroutine itself runs in the calling goroutine,
// before the `go` statement spawns anything), and returns a closure that
// installs the propagated stacks on the new goroutine before running body.
//
// This is not meant to be called directly by user code.
//
// Note: because body is a closure over the original call's arguments rather
// than a call with its own freshly-evaluated arguments, those arguments are
// now evaluated when body executes (i.e. on the new goroutine) rather than
// before the goroutine is spawned, which differs from plain Go's `go f(x)`
// semantics. This is an accepted trade-off of rewriting every `go` statement
// generically: preserving exact evaluation order would require knowing the
// static types of f's arguments at every call site.
func WrapGoroutine(body func()) func() {
	if !enabled() {
		return body
	}

	parent := getBlob()
	entries := registrySnapshot()

	return func() {
		child := make([]any, len(entries))
		for i, e := range entries {
			if e == nil {
				continue
			}
			var p any
			if i < len(parent) {
				p = parent[i]
			}
			child[i] = e.goHook(p)
		}
		setBlob(child)
		body()
	}
}

// Bootstrap dispatches [Hooks.Main] across every registered type and
// installs the resulting stacks on the calling goroutine. It is woven by
// orchestrion into the very start of the program's `func main()`, and is
// not meant to be called directly by user code.
func Bootstrap() {
	if !enabled() {
		return
	}

	entries := registrySnapshot()
	blob := make([]any, len(entries))
	for i, e := range entries {
		if e == nil {
			continue
		}
		blob[i] = e.main()
	}
	setBlob(blob)
}
