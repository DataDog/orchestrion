// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package context

// Stack is a generic stack type that can be used to store values of any
// single type.
type Stack[T any] []T

// Hooks allows users to hook into the execution of a program at points where
// context propagation would matter, to define custom behavior for context
// propagation. All parameters passed in can be nil if no context was set at
// all on the relevant goroutine beforehand.
type Hooks[T any] interface {
	// Main is called before the main function of the program is executed.
	Main() *Stack[T]

	// Go is called synchronously by runtime.newproc on the parent goroutine,
	// after the go statement's callee and arguments are evaluated. It returns
	// the stack that will be installed before the child becomes runnable.
	//
	// It is also dispatched from runtime.newcoro, so the goroutine backing an
	// iterator pulled via iter.Pull/iter.Pull2 inherits the creating
	// goroutine's context the same way.
	//
	// The returned stack must not alias parent (or any stack reachable from
	// it): the parent keeps using its own stack concurrently, so returning it
	// directly (or a slice sharing its backing array) makes concurrent Push and
	// Pop a data race. Return a fresh copy of the values to propagate instead.
	// Implementations must be non-blocking, lock-free, and bounded-allocation.
	// Go runs while the runtime duplicates the parent g. Goroutine creations
	// that happen with runtime locks held (e.g. background GC workers) skip
	// propagation entirely instead of dispatching the hook: a hook that blocks
	// or parks there would kill the process. Runtime code spawning a goroutine
	// on an unlocked user goroutine (e.g. runtime.AddCleanup's cleanup worker)
	// does dispatch the hook on the parent, but the resulting stack is not
	// installed: goroutines the runtime owns never inherit context.
	Go(parent *Stack[T]) *Stack[T]

	// ChanRecv is called when a value is received from a channel that was
	// opted into propagation (see [NewChan]). It receives the receiving
	// goroutine's own stack and the stack captured by the corresponding
	// [Hooks.ChanSend] call, and returns the stack to install on the
	// receiving goroutine.
	//
	// The returned stack must not alias parent: the receiving goroutine keeps
	// using its own stack concurrently. The sent stack is owned by the
	// receiving side after the call, so returning it (or a copy) is safe.
	ChanRecv(parent *Stack[T], sent *Stack[T]) *Stack[T]

	// ChanSend is called when a value is sent on a channel that was opted
	// into propagation (see [NewChan]). The returned stack is delivered to
	// the corresponding [Hooks.ChanRecv] call as its `sent` argument.
	//
	// The returned stack must not alias parent (or any stack reachable from
	// it): the sending goroutine keeps using its own stack after the send, so
	// returning it directly makes concurrent Push and Pop a data race. Return
	// a fresh copy of the values to propagate instead.
	ChanSend(parent *Stack[T]) *Stack[T]
}

// Controller allows pushing and popping values onto the current goroutine's
// context stack for a single registered type T.
type Controller[T any] struct {
	slot *slot[T]
}

// Push adds val to the top of the current goroutine's stack for T.
func (c *Controller[T]) Push(val T) {
	if c == nil || !enabled() {
		return
	}
	st := c.slot.current()
	*st = append(*st, val)
}

// Pop removes and returns the value at the top of the current goroutine's
// stack for T. It returns the zero value of T if the stack is empty.
//
// Unlike [Controller.Peek], Pop does not return a found flag: it is meant
// for the balanced Push/Pop pattern where the caller already knows a value
// is present (it pushed one itself) and does not need to distinguish a
// genuine zero value from an empty stack. Use [Controller.Peek] instead
// when that distinction matters, e.g. to decide whether to push at all.
func (c *Controller[T]) Pop() T {
	var zero T
	if c == nil || !enabled() {
		return zero
	}
	st := c.slot.current()
	n := len(*st)
	if n == 0 {
		return zero
	}
	val := (*st)[n-1]
	(*st)[n-1] = zero // avoid retaining a reference in the backing array
	*st = (*st)[:n-1]
	return val
}

// Peek returns the value at the top of the current goroutine's stack for T,
// without removing it, along with whether a value was present.
func (c *Controller[T]) Peek() (T, bool) {
	var zero T
	if c == nil || !enabled() {
		return zero, false
	}
	st := c.slot.current()
	n := len(*st)
	if n == 0 {
		return zero, false
	}
	return (*st)[n-1], true
}

// Register registers hooks for the given type T and returns a [Controller]
// that can be used to push and pull values from the current goroutine's
// stack for T. The context key used to track this propagation is the type T
// itself: calling Register more than once for the same T creates
// independent, non-interacting registrations.
//
// Register is expected to be called at init time, before any goroutine that
// needs propagation is created.
func Register[T any](hooks Hooks[T]) *Controller[T] {
	s := newSlot(hooks)
	// Publish only after the new registry snapshot is visible. This is an
	// atomic runtime-side store when woven and a no-op otherwise.
	enableGoroutinePropagation()
	return &Controller[T]{slot: s}
}
