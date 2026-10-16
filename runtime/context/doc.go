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
// Importing this package and calling [Register] is not by itself enough to
// enable propagation: the aspects that add storage and runtime hooks (see
// runtime/context/orchestrion.yml) are opt-in, and only get loaded when the
// consuming project's own orchestrion.tool.go blank-imports this package:
//
//	//go:build tools
//	package tools
//
//	import (
//		_ "github.com/DataDog/orchestrion"
//		_ "github.com/DataDog/orchestrion/runtime/context"
//	)
//
// Without that blank import, Register still returns a Controller, but its
// methods and propagation remain disabled because runtime.g has no storage or
// runtime.newproc hook.
//
// A consumer (typically a tracer) calls Register once, usually at init time,
// with a Hooks implementation for the value type T it wants to propagate. The
// returned Controller is used to Push a value before entering code that may
// spawn goroutines and Pop it when that section ends.
//
// Once at least one hook is registered, the instrumented runtime.newproc
// dispatches Hooks.Go synchronously on the parent goroutine. Native Go has
// already evaluated the go statement's callee and arguments at that point, and
// the child is not runnable yet. The resulting stacks are installed directly
// on the child runtime.g before it is queued, preserving ordinary evaluation
// order and panic behavior.
//
// # Scope of automatic propagation
//
// Only goroutine creation (go statements) is automatically instrumented.
// Channel-based propagation ([Hooks.ChanSend], [Hooks.ChanRecv]) is implemented
// and callable, but is deliberately not woven into plain channel operations.
// Exact correlation under concurrent senders, receivers, and select requires
// changes throughout the runtime channel implementation. Use [NewChan] to opt
// a specific channel into exact propagation instead.
package context
