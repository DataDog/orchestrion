// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package context

// mutex is a minimal binary mutex built purely on the chan language primitive,
// so this low-level package does not need to import "sync". Keeping the
// dependency closure small is important because runtime reaches propagation
// through a reverse callback.
type mutex chan struct{}

func newMutex() mutex {
	m := make(mutex, 1)
	m <- struct{}{}
	return m
}

func (m mutex) Lock()   { <-m }
func (m mutex) Unlock() { m <- struct{}{} }
