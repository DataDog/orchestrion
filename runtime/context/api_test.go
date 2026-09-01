// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package context

import (
	"runtime"
	"strconv"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockGLS enables this package's propagation machinery for the duration of
// a test, without requiring the test binary to have been built with
// orchestrion, mirroring
// github.com/DataDog/orchestrion/runtime/context/contexttest.MockGLS (kept
// as a local, package-internal duplicate here rather than imported, since
// this file is white-box (package context) and contexttest imports
// context -- see registry.go and mutex.go for why context itself must not
// import "sync"/"runtime"/"strconv", which don't apply to _test.go files
// like this one).
func mockGLS() (cleanup func()) {
	var stacks sync.Map // goroutine ID -> any
	return SetGLSForTesting(
		func() any {
			val, _ := stacks.Load(goroutineID())
			return val
		},
		func(v any) {
			stacks.Store(goroutineID(), v)
		},
	)
}

func goroutineID() uint64 {
	var buf [64]byte
	n := runtime.Stack(buf[:], false)
	s := buf[len("goroutine "):n]
	for i, b := range s {
		if b == ' ' {
			s = s[:i]
			break
		}
	}
	id, _ := strconv.ParseUint(string(s), 10, 64)
	return id
}

// stringHooks is a minimal [Hooks] implementation for tests: it just copies
// the parent stack (or an empty one) so propagation is directly observable.
type stringHooks struct{}

func (stringHooks) Main() *Stack[string] { return &Stack[string]{"main"} }

func (stringHooks) Go(parent *Stack[string]) *Stack[string] {
	if parent == nil {
		return new(Stack[string])
	}
	cp := make(Stack[string], len(*parent))
	copy(cp, *parent)
	return &cp
}

func (stringHooks) ChanSend(parent *Stack[string]) *Stack[string] {
	return stringHooks{}.Go(parent)
}

func (stringHooks) ChanRecv(_ *Stack[string], sent *Stack[string]) *Stack[string] {
	if sent == nil {
		return new(Stack[string])
	}
	cp := make(Stack[string], len(*sent))
	copy(cp, *sent)
	return &cp
}

func TestControllerPushPopWithoutOrchestrion(t *testing.T) {
	// Without MockGLS, enabled() is false: everything should be a silent no-op.
	ctrl := Register[string](stringHooks{})
	ctrl.Push("a")
	v, ok := ctrl.Peek()
	assert.False(t, ok)
	assert.Equal(t, "", v)
	assert.Equal(t, "", ctrl.Pop())
}

func TestControllerPushPop(t *testing.T) {
	defer mockGLS()()

	ctrl := Register[string](stringHooks{})

	_, ok := ctrl.Peek()
	require.False(t, ok)

	ctrl.Push("a")
	ctrl.Push("b")

	v, ok := ctrl.Peek()
	require.True(t, ok)
	assert.Equal(t, "b", v)

	assert.Equal(t, "b", ctrl.Pop())
	assert.Equal(t, "a", ctrl.Pop())
	assert.Equal(t, "", ctrl.Pop()) // empty stack returns zero value
}

type intHooks struct{}

func (intHooks) Main() *Stack[int]                        { return new(Stack[int]) }
func (intHooks) Go(parent *Stack[int]) *Stack[int]        { return parent }
func (intHooks) ChanSend(parent *Stack[int]) *Stack[int]  { return parent }
func (intHooks) ChanRecv(_, sent *Stack[int]) *Stack[int] { return sent }

func TestMultipleRegistrationsAreIndependent(t *testing.T) {
	defer mockGLS()()

	strCtrl := Register[string](stringHooks{})
	intCtrl := Register[int](intHooks{})

	strCtrl.Push("hello")
	intCtrl.Push(42)

	// Each registration's stack is independent: popping one must not
	// disturb the other.
	assert.Equal(t, 42, intCtrl.Pop())
	assert.Equal(t, "hello", strCtrl.Pop())
}

func TestWrapGoroutinePropagatesToChild(t *testing.T) {
	defer mockGLS()()

	ctrl := Register[string](stringHooks{})
	ctrl.Push("root")
	ctrl.Push("child-span")

	done := make(chan string, 1)

	// This is exactly the shape orchestrion's advice weaves in place of a
	// `go` statement: `go f()` becomes `go WrapGoroutine(func() { f() })()`.
	go WrapGoroutine(func() {
		v, ok := ctrl.Peek()
		if !ok {
			done <- "<none>"
			return
		}
		done <- v
	})()

	assert.Equal(t, "child-span", <-done)

	// The parent goroutine's own stack must be unaffected by the child's.
	assert.Equal(t, "child-span", ctrl.Pop())
	assert.Equal(t, "root", ctrl.Pop())
}

func TestWrapGoroutineDisabledIsPlainGo(t *testing.T) {
	// No MockGLS: enabled() is false, so WrapGoroutine must degrade to
	// returning body unchanged.
	ctrl := Register[string](stringHooks{})
	ctrl.Push("root")

	done := make(chan bool, 1)
	go WrapGoroutine(func() {
		_, ok := ctrl.Peek()
		done <- ok
	})()

	assert.False(t, <-done)
}

func TestBootstrapSeedsMainStack(t *testing.T) {
	defer mockGLS()()

	ctrl := Register[string](stringHooks{})
	Bootstrap()

	assert.Equal(t, "main", ctrl.Pop())
}

func TestChanSendRecvPropagates(t *testing.T) {
	defer mockGLS()()

	ctrl := Register[string](stringHooks{})
	raw := make(chan int)
	ch := NewChan(raw)

	var wg sync.WaitGroup
	wg.Add(1)

	results := make(chan string, 1)
	go func() {
		defer wg.Done()
		v, ok := ch.Recv()
		require.True(t, ok)
		assert.Equal(t, 42, v)
		got, _ := ctrl.Peek()
		results <- got
	}()

	ctrl.Push("sender-span")
	ch.Send(42)

	wg.Wait()
	assert.Equal(t, "sender-span", <-results)
}

func TestChanCloseStopsRecv(t *testing.T) {
	defer mockGLS()()

	raw := make(chan int)
	ch := NewChan(raw)
	ch.Close()

	_, ok := ch.Recv()
	assert.False(t, ok)
}

func TestChanDisabledBehavesLikePlainChannel(t *testing.T) {
	raw := make(chan int, 1)
	ch := NewChan(raw)

	ch.Send(7)
	v, ok := ch.Recv()
	require.True(t, ok)
	assert.Equal(t, 7, v)
}
