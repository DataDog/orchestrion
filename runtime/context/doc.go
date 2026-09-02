// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

// Package context implements goroutine context propagation: it lets a value
// pushed on one goroutine be observed by a goroutine spawned from it, without
// requiring the value to be threaded through every intervening function
// signature.
//
// # Usage
//
// A consumer (typically a tracer) calls [Register] once, at init time, with
// a [Hooks] implementation for the value type T it wants to propagate. The
// returned [Controller] is then used to [Controller.Push] a value before
// entering a section of code that may spawn goroutines, and [Controller.Pop]
// it when that section ends.
//
// Once registered, every `go` statement compiled into the program by
// orchestrion is woven to call [Hooks.Go] with the calling goroutine's
// current [Stack], installing the result as the new goroutine's [Stack]
// before it starts running.
//
// A specific `go` statement can be excluded from this rewrite by placing a
// `//orchestrion:ignore` comment immediately above it, which restores exact
// plain-Go evaluation order (callee and arguments evaluated on the calling
// goroutine, before it spawns) for that one statement. See [WrapGoroutine]'s
// doc comment for why this matters, and the general
// `//orchestrion:ignore` documentation for how the directive behaves.
//
// # Scope of automatic propagation
//
// Only goroutine creation (`go` statements) is automatically instrumented.
// Channel-based propagation ([Hooks.ChanSend], [Hooks.ChanRecv]) is
// implemented and callable, but is deliberately not woven automatically into
// plain `chan` send/receive operations: doing so without changing the
// channel's type would require correlating a side-channel payload with the
// exact value observed by a receiver purely in user-space, which cannot be
// made correct under concurrent multi-sender/multi-receiver use of the same
// channel without patching the runtime's channel implementation directly
// (the same limitation called out as an open TODO in the original design).
// Use [NewChan] to opt a specific channel into propagation instead.
package context
