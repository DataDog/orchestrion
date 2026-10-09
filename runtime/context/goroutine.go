// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package context

// propagateGoroutine is installed into the instrumented runtime after the
// first Register call. runtime.newproc calls it synchronously on the parent
// goroutine, after the compiler has evaluated the go statement's callee and
// arguments and before switching to g0 to create the child.
//
// The argument and result are type-erased because runtime cannot import this
// package. Both are context blobs: slices indexed by registration slot.
func propagateGoroutine(parentValue any) any {
	parent, _ := parentValue.([]any)
	entries := registrySnapshot()

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
	return child
}

// Bootstrap dispatches [Hooks.Main] across every registered type and
// installs the resulting stacks on the calling goroutine. It is woven by
// orchestrion into the very start of the program's func main, and is not
// meant to be called directly by user code.
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
