// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package context

// Stack is a generic stack type that can be used to store values of any type.
type Stack[T any] []T

// Hooks is an interface that allows users to hook into the execution of a program when context propagation would matter
// to define a custom behavior for context propagation.
type Hooks[T any] interface {
	// Main is called before the main function of the program is executed.
	Main() *Stack[T]

	// Go is called when a new goroutine is created. // It receives the parent stack and returns a new stack for the goroutine.
	Go(parent *Stack[T]) *Stack[T]

	// ChanRecv is called when a value is received from a channel.
	// It receives the parent stack and the stack of the sender goroutine, and returns a new stack for the receiving goroutine.
	ChanRecv(parent *Stack[T], sent *Stack[T]) *Stack[T]

	// ChanSend is called when a value is sent to a channel. The returned value will be passed as the `sent` argument to ChanRecv.
	ChanSend(parent *Stack[T]) *Stack[T]
}

// Controller is an interface to push and pull values from the current context stack.
type Controller[T any] struct {
	// ...
}

func (c *Controller[T]) Push(_ T) {
}

func (c *Controller[T]) Pop() T {
	var x T
	return x
}

// Register registers the hooks for the given type T and returns a Controller[T] that can be used to push and pull values from the current stack.
// The context key used to track this propagation is the type T itself
func Register[T any](_ Hooks[T]) *Controller[T] {
	return nil
}
