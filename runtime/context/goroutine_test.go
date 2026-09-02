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

// recoverPanic runs f and returns whatever value it panicked with, or nil if
// f returned normally. It lets the tests below assert on the exact
// recovered value (its type, its Error() message, its Unwrap chain...)
// without needing to spawn a real goroutine: calling the closure
// [WrapGoroutine] returns synchronously, from the test's own goroutine, is
// enough to exercise the same recover/re-panic logic it runs on a real one.
func recoverPanic(f func()) (recovered any) {
	defer func() {
		recovered = recover()
	}()
	f()
	return nil
}

// TestWrapGoroutineWrapsErrorPanic verifies that, with propagation enabled,
// a panic with an error value is recovered and re-panicked as a new error
// whose message still contains the original one, and which remains
// errors.Is-reachable back to the original error (the equivalent of
// fmt.Errorf's "%w", hand-rolled by [goroutinePanic] since this package
// cannot import "fmt" -- see [goroutinePanic]'s doc comment).
func TestWrapGoroutineWrapsErrorPanic(t *testing.T) {
	defer mockGLS()()

	original := errors.New("boom")
	wrapped := WrapGoroutine(func() { panic(original) })

	recovered := recoverPanic(wrapped)
	require.NotNil(t, recovered, "wrapped closure must re-panic")

	recoveredErr, ok := recovered.(error)
	require.True(t, ok, "recovered panic value must be an error, got %T", recovered)

	assert.Contains(t, recoveredErr.Error(), "boom", "wrapped message must still contain the original message")
	assert.True(t, errors.Is(recoveredErr, original), "errors.Is must be able to reach the original error")
	assert.Same(t, original, errors.Unwrap(recoveredErr), "errors.Unwrap must yield the exact original error value")
}

// TestWrapGoroutineWrapsNonErrorPanic verifies that, with propagation
// enabled, a panic with a non-error value (a bare string here) is still
// recovered and re-panicked with a message that contains the original
// value's text, even though -- unlike the error case -- there is no
// Unwrap path back to it (there is nothing to wrap: fmt.Errorf's "%v" verb
// has no equivalent unwrap mechanism either).
func TestWrapGoroutineWrapsNonErrorPanic(t *testing.T) {
	defer mockGLS()()

	wrapped := WrapGoroutine(func() { panic("boom") })

	recovered := recoverPanic(wrapped)
	require.NotNil(t, recovered, "wrapped closure must re-panic")

	recoveredErr, ok := recovered.(error)
	require.True(t, ok, "recovered panic value must be an error, got %T", recovered)

	assert.Contains(t, recoveredErr.Error(), "boom", "wrapped message must still contain the original panic text")
	assert.Nil(t, errors.Unwrap(recoveredErr), "a non-error panic value has nothing to unwrap")
}

// TestWrapGoroutineDisabledPropagatesOriginalPanicUnwrapped verifies that,
// with propagation disabled, WrapGoroutine's fast path (return body
// unchanged) is untouched by the recover/re-panic behavior added above: the
// closure it returns is body itself, so a panic from body propagates
// exactly as-is, with no wrapping at all.
func TestWrapGoroutineDisabledPropagatesOriginalPanicUnwrapped(t *testing.T) {
	if enabled() {
		t.Fatal("propagation must be disabled for this test, but a previous test left the mock GLS state enabled (missing restore()?)")
	}

	original := errors.New("boom")
	wrapped := WrapGoroutine(func() { panic(original) })

	recovered := recoverPanic(wrapped)
	require.NotNil(t, recovered, "body must still panic")

	assert.Same(t, original, recovered, "the disabled path must propagate the exact original panic value, unwrapped")

	if _, ok := recovered.(*goroutinePanic); ok {
		t.Fatal("the disabled path must not wrap the panic in *goroutinePanic")
	}
}

// TestWrapGoroutineCapturesParentStackSynchronously verifies that
// WrapGoroutine dispatches Hooks.Go against the parent's stack
// synchronously, at the time WrapGoroutine itself is called -- not lazily
// inside the closure it returns. The documented Push/spawn/Pop pattern (see
// doc.go) is `Push(v); go work(); Pop()`: if Hooks.Go were dispatched lazily
// inside the closure instead, a Pop that runs before the spawned goroutine
// is actually scheduled would empty the very same backing Stack[T] object
// the closure's dispatch reads from, and the child would silently observe
// a stale, empty stack.
//
// This deliberately does not spawn a real goroutine or use any
// channel/WaitGroup synchronization: real scheduling is nondeterministic,
// and synchronizing before Pop (like TestWrapGoroutinePropagatesToChild
// does) would incidentally prevent the exact race this test needs to
// catch. Instead, it calls WrapGoroutine, Pops immediately afterwards
// (before ever invoking the returned closure), and only then invokes the
// closure synchronously in the test's own goroutine -- the same technique
// recoverPanic uses above. Because the closure's setBlob installs `child`
// on whichever goroutine calls it, and because body reads back through
// Controller.Peek on that same goroutine, this deterministically observes
// whether Hooks.Go actually ran before or after the Pop.
func TestWrapGoroutineCapturesParentStackSynchronously(t *testing.T) {
	defer mockGLS()()

	ctrl := Register[string](stringHooks{})
	ctrl.Push("span")

	var observed string
	var ok bool
	wrapped := WrapGoroutine(func() {
		observed, ok = ctrl.Peek()
	})

	// Pop immediately after obtaining the closure, before ever invoking
	// it. With the bug, this empties the same Stack[string] object the
	// closure's (still-pending) Hooks.Go dispatch would read from.
	popped := ctrl.Pop()
	require.Equal(t, "span", popped, "sanity: the value must actually have been on the stack to pop")

	wrapped()

	assert.True(t, ok, "child must still see the value pushed by the parent before Pop, even though Pop already ran before the closure was invoked")
	assert.Equal(t, "span", observed, "child must observe the stack snapshotted at WrapGoroutine-call time, not whatever the parent's stack holds by the time the closure runs")
}
