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
