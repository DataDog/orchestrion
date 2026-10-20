// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package context

// Chan wraps a plain Go channel to propagate context stacks from each
// [Chan.Send] call to the corresponding [Chan.Recv] call, dispatching
// [Hooks.ChanSend] and [Hooks.ChanRecv] in the process.
//
// Unlike goroutine creation, channel operations on a plain `chan T` are not
// automatically instrumented: correlating a side payload with the exact
// value a receiver observes cannot be done correctly, in user space, under
// concurrent multi-sender/multi-receiver use of the same channel without
// changing the channel's type (see the package doc). Chan sidesteps this by
// carrying the context stacks on a second, internal channel and serializing
// each side with a mutex so that a stack snapshot and its corresponding
// value are always sent, and received, as one indivisible pair -- this
// makes propagation exact rather than best-effort, at the cost of pairwise
// serializing sends (and receives) that would otherwise race freely on the
// underlying channel.
//
// A zero-value Chan (i.e. one not obtained from [NewChan]) is not valid:
// its first use via [Chan.Send], [Chan.Recv], or [Chan.Close] panics
// immediately instead of silently deadlocking on a nil mutex or a nil
// underlying channel.
type Chan[T any] struct {
	data chan T

	ctx     chan []any
	sendMu  mutex
	recvMu  mutex
	closeMu mutex
	closed  bool
}

// NewChan wraps data for context propagation. data should not be sent to or
// received from directly once wrapped, other than through the returned
// [Chan]; doing so bypasses propagation and, for sends, will desynchronize
// the internal pairing between data and the context stacks and pointer.
//
// A [Chan] must be constructed via NewChan: a zero-value Chan panics on
// first use rather than deadlocking silently.
func NewChan[T any](data chan T) *Chan[T] {
	return &Chan[T]{
		data:    data,
		ctx:     make(chan []any, cap(data)),
		sendMu:  newMutex(),
		recvMu:  newMutex(),
		closeMu: newMutex(),
	}
}

// Send sends v on the wrapped channel, first dispatching [Hooks.ChanSend]
// against the calling goroutine's current stacks and pairing the result
// with v.
func (c *Chan[T]) Send(v T) {
	if c.sendMu == nil {
		panic("context.Chan[T]: zero-value Chan is not valid; construct one with NewChan")
	}

	if !enabled() {
		c.data <- v
		return
	}

	c.sendMu.Lock()
	defer c.sendMu.Unlock()

	parent := getBlob()
	entries := registrySnapshot()
	sent := make([]any, len(entries))
	for i, e := range entries {
		if e == nil {
			continue
		}
		var p any
		if i < len(parent) {
			p = parent[i]
		}
		sent[i] = e.chanSend(p)
	}

	c.ctx <- sent
	c.data <- v
}

// Recv receives a value from the wrapped channel, then dispatches
// [Hooks.ChanRecv] against the receiving goroutine's current stacks and the
// stacks captured by the paired [Chan.Send] call, installing the result on
// the receiving goroutine. The second return value is false if the channel
// was closed and drained, mirroring the two-value form of a plain channel
// receive.
func (c *Chan[T]) Recv() (T, bool) {
	if c.recvMu == nil {
		panic("context.Chan[T]: zero-value Chan is not valid; construct one with NewChan")
	}

	if !enabled() {
		v, ok := <-c.data
		return v, ok
	}

	c.recvMu.Lock()
	defer c.recvMu.Unlock()

	sent, ok := <-c.ctx
	if !ok {
		var zero T
		return zero, false
	}

	v, ok := <-c.data
	if !ok {
		var zero T
		return zero, false
	}

	parent := getBlob()
	entries := registrySnapshot()
	child := make([]any, len(entries))
	for i, e := range entries {
		if e == nil {
			continue
		}
		var p, s any
		if i < len(parent) {
			p = parent[i]
		}
		if i < len(sent) {
			s = sent[i]
		}
		child[i] = e.chanRecv(p, s)
	}
	setBlob(child)

	return v, true
}

// Close closes the wrapped channel. It is safe to call at most once, and
// must only be called by the sole sender, exactly like a plain channel.
func (c *Chan[T]) Close() {
	if c.closeMu == nil {
		panic("context.Chan[T]: zero-value Chan is not valid; construct one with NewChan")
	}

	c.closeMu.Lock()
	defer c.closeMu.Unlock()
	if c.closed {
		return
	}
	c.closed = true
	close(c.ctx)
	close(c.data)
}
