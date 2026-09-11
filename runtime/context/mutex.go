// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package context

// mutex is a minimal binary mutex built purely on the `chan` language
// primitive, so this package does not need to import "sync". [WrapGoroutine]
// is woven into every `go` statement orchestrion compiles, including ones
// inside sync's own transitive dependencies; importing sync here would risk
// an import cycle invisible to Go's static build graph (it only appears once
// toolexec starts rewriting `go` statements at compile time). See
// [registry] for the same reasoning applied to the registration slice.
type mutex chan struct{}

func newMutex() mutex {
	m := make(mutex, 1)
	m <- struct{}{}
	return m
}

func (m mutex) Lock()   { <-m }
func (m mutex) Unlock() { m <- struct{}{} }
