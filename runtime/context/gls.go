// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package context

import (
	_ "unsafe" // for go:linkname
)

// getGLS and setGLS read/write the current goroutine's context blob. They
// default to a disabled no-op pair, and are switched to the real
// runtime-backed implementation in init if the program was built with
// orchestrion (see below). Tests may swap them via [SetGLSForTesting], or
// the higher-level github.com/DataDog/orchestrion/runtime/context/contexttest
// package.
var (
	getGLS    = func() any { return nil }
	setGLS    = func(any) {}
	isEnabled = false
)

func enabled() bool {
	return isEnabled
}

// SetGLSForTesting overrides this package's per-goroutine storage with get
// and set for the duration of a test, without requiring the test binary to
// have been built with orchestrion, and returns a function that restores
// the previous state.
//
// This is a low-level hook: get/set are expected to already provide
// per-goroutine isolation (e.g. keyed by a goroutine ID) if the test
// exercises goroutine-crossing behavior. Most callers should prefer
// github.com/DataDog/orchestrion/runtime/context/contexttest.MockGLS, which
// implements that isolation. This function exists (with a name distinct
// from a real per-goroutine mock) specifically so this package itself does
// not need to import anything beyond "unsafe" to support it -- see
// [registry] and [mutex] for why that matters.
func SetGLSForTesting(get func() any, set func(any)) (restore func()) {
	prevGet, prevSet, prevEnabled := getGLS, setGLS, isEnabled
	getGLS, setGLS, isEnabled = get, set, true
	return func() {
		getGLS, setGLS, isEnabled = prevGet, prevSet, prevEnabled
	}
}

// __dd_orchestrion_ctx_get and __dd_orchestrion_ctx_set are populated by the
// orchestrion builtin aspect that instruments the standard library's
// runtime package, which injects a struct field on runtime.g together with
// matching accessor closures exposed under these link names. They remain
// nil when the program was not built with orchestrion.
//
// These accessors reach storage via getg().m.curg.<field>, resolving to the
// currently *running* g. A separate aspect (context.gls.scrub, woven into
// runtime.goexit1; see runtime/context/orchestrion.yml) instead writes via
// getg().<field> directly, with no .m.curg. The two forms are equivalent
// only inside goexit1, where getg() already is curg; they are not
// interchangeable in general. Any accessor added elsewhere must pick the
// form that matches whether it acts on an arbitrary g's behalf or on the
// currently-running g.
//
//go:linkname __dd_orchestrion_ctx_get __dd_orchestrion_ctx_get
var __dd_orchestrion_ctx_get func() any

//go:linkname __dd_orchestrion_ctx_set __dd_orchestrion_ctx_set
var __dd_orchestrion_ctx_set func(any)

func init() {
	if __dd_orchestrion_ctx_get != nil && __dd_orchestrion_ctx_set != nil {
		getGLS = __dd_orchestrion_ctx_get
		setGLS = __dd_orchestrion_ctx_set
		isEnabled = true
	}
}

// getBlob returns the current goroutine's context blob: a slice indexed by
// each [Register] call's assigned slot index.
func getBlob() []any {
	blob, _ := getGLS().([]any)
	return blob
}

// setBlob installs blob as the current goroutine's context blob.
func setBlob(blob []any) {
	setGLS(blob)
}

// getBlobEntry returns the current goroutine's blob entry at index, or nil
// if the blob is not large enough to hold it yet.
func getBlobEntry(index int) any {
	blob := getBlob()
	if index >= len(blob) {
		return nil
	}
	return blob[index]
}

// setBlobEntry sets the current goroutine's blob entry at index, growing the
// blob as needed.
func setBlobEntry(index int, val any) {
	blob := getBlob()
	if index >= len(blob) {
		blob = append(blob, make([]any, index+1-len(blob))...)
	}
	blob[index] = val
	setBlob(blob)
}
