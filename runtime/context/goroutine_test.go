// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package context

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type panicGoHooks struct {
	stringHooks
	value any
}

func (h panicGoHooks) Go(*Stack[string]) *Stack[string] { panic(h.value) }

// TestPropagateGoroutineCapturesParentStackSynchronously exercises the
// callback runtime.newproc invokes on the parent before switching to g0.
func TestPropagateGoroutineCapturesParentStackSynchronously(t *testing.T) {
	restore := mockGLS()
	defer restore()
	oldRegistry := registry.Load()
	registry.Store(nil)
	defer registry.Store(oldRegistry)

	ctrl := Register[string](stringHooks{})
	ctrl.Push("span")

	child := propagateGoroutine(getBlob())
	require.Equal(t, "span", ctrl.Pop())

	setBlob(child.([]any))
	observed, ok := ctrl.Peek()
	assert.True(t, ok)
	assert.Equal(t, "span", observed)
}

// TestPropagateGoroutineDoesNotWrapHookPanic verifies that a Hooks.Go panic
// remains the original panic value. runtime.newproc invokes this callback in
// the parent, matching native go-statement evaluation behavior.
func TestPropagateGoroutineDoesNotWrapHookPanic(t *testing.T) {
	restore := mockGLS()
	defer restore()
	oldRegistry := registry.Load()
	registry.Store(nil)
	defer registry.Store(oldRegistry)

	original := errors.New("boom")
	Register[string](panicGoHooks{value: original})

	var recovered any
	func() {
		defer func() { recovered = recover() }()
		propagateGoroutine(getBlob())
	}()

	assert.Same(t, original, recovered)
}
