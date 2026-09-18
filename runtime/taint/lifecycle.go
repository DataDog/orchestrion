// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package taint

import (
	"bufio"
	"strings"
	"sync"
	"unsafe"
)

type registryLifecycle struct {
	mu               sync.RWMutex
	current          *registry
	retired          []*registry
	historyDiscarded bool
}

const maxRetiredGenerations = 1

func newRegistryLifecycle() registryLifecycle {
	current := newRegistry()
	return registryLifecycle{current: &current}
}

// StartRequest starts a logical request generation while retaining one prior generation.
func StartRequest() {
	active.mu.Lock()
	if len(active.retired) >= maxRetiredGenerations {
		active.retired = active.retired[1:]
		active.historyDiscarded = true
	}
	active.retired = append(active.retired, active.current)
	current := newRegistry()
	active.current = &current
	active.mu.Unlock()
}

func (lifecycle *registryLifecycle) currentRegistry() *registry {
	lifecycle.mu.RLock()
	current := lifecycle.current
	lifecycle.mu.RUnlock()
	return current
}

func (lifecycle *registryLifecycle) snapshot() []*registry {
	registries, _ := lifecycle.snapshotAndHistory()
	return registries
}

func (lifecycle *registryLifecycle) snapshotAndHistory() ([]*registry, bool) {
	lifecycle.mu.RLock()
	registries := make([]*registry, len(lifecycle.retired), len(lifecycle.retired)+1)
	copy(registries, lifecycle.retired)
	registries = append(registries, lifecycle.current)
	historyDiscarded := lifecycle.historyDiscarded
	lifecycle.mu.RUnlock()
	return registries, historyDiscarded
}

func (lifecycle *registryLifecycle) stringRangesAndSaturation(value string) ([]Range, bool) {
	start, ok := stringAddress(value)
	var result []Range
	registries, saturated := lifecycle.snapshotAndHistory()
	for _, registry := range registries {
		registry.mu.RLock()
		if ok {
			result = append(result, relativeRanges(registry.stringRanges, start, len(value))...)
		}
		saturated = saturated || registry.stateSaturated
		registry.mu.RUnlock()
	}
	return normalizeRanges(result, len(value)), saturated
}

func (lifecycle *registryLifecycle) byteRanges(start uintptr, length int) []Range {
	var result []Range
	for _, registry := range lifecycle.snapshot() {
		registry.mu.RLock()
		result = append(result, relativeRanges(registry.byteRanges, start, length)...)
		registry.mu.RUnlock()
	}
	return normalizeRanges(result, length)
}

func (lifecycle *registryLifecycle) runeRanges(start uintptr, length int) []Range {
	var result []Range
	for _, registry := range lifecycle.snapshot() {
		registry.mu.RLock()
		result = append(result, relativeRanges(registry.runeRanges, start, length)...)
		registry.mu.RUnlock()
	}
	return normalizeRanges(result, length)
}

func (lifecycle *registryLifecycle) registryForBuilder(builder *strings.Builder) *registry {
	for _, registry := range lifecycle.snapshot() {
		registry.mu.RLock()
		_, tracked := registry.builders[builder]
		registry.mu.RUnlock()
		if tracked {
			return registry
		}
	}
	return lifecycle.currentRegistry()
}

func (lifecycle *registryLifecycle) registryForStringReader(reader *strings.Reader) *registry {
	for _, registry := range lifecycle.snapshot() {
		registry.mu.RLock()
		_, tracked := registry.stringReaders[reader]
		registry.mu.RUnlock()
		if tracked {
			return registry
		}
	}
	return lifecycle.currentRegistry()
}

func (lifecycle *registryLifecycle) registryForBufioReader(reader *bufio.Reader) *registry {
	for _, registry := range lifecycle.snapshot() {
		registry.mu.RLock()
		_, tracked := registry.bufioReaders[reader]
		registry.mu.RUnlock()
		if tracked {
			return registry
		}
	}
	return lifecycle.currentRegistry()
}

func (lifecycle *registryLifecycle) byteScalarTainted(address unsafe.Pointer) bool {
	for _, registry := range lifecycle.snapshot() {
		registry.mu.RLock()
		_, tainted := registry.byteScalars[address]
		registry.mu.RUnlock()
		if tainted {
			return true
		}
	}
	return false
}

func (lifecycle *registryLifecycle) runeScalarTainted(address unsafe.Pointer) bool {
	for _, registry := range lifecycle.snapshot() {
		registry.mu.RLock()
		_, tainted := registry.runeScalars[address]
		registry.mu.RUnlock()
		if tainted {
			return true
		}
	}
	return false
}

func (lifecycle *registryLifecycle) setByteScalarTaint(address unsafe.Pointer, tainted bool) {
	lifecycle.setScalarTaint(address, tainted, true)
}

func (lifecycle *registryLifecycle) setRuneScalarTaint(address unsafe.Pointer, tainted bool) {
	lifecycle.setScalarTaint(address, tainted, false)
}

func (lifecycle *registryLifecycle) setScalarTaint(address unsafe.Pointer, tainted, byteScalar bool) {
	for _, registry := range lifecycle.snapshot() {
		registry.mu.Lock()
		var tracked bool
		if byteScalar {
			_, tracked = registry.byteScalars[address]
		} else {
			_, tracked = registry.runeScalars[address]
		}
		if tracked {
			if byteScalar {
				if tainted {
					registry.byteScalars[address] = struct{}{}
				} else {
					delete(registry.byteScalars, address)
				}
			} else if tainted {
				registry.runeScalars[address] = struct{}{}
			} else {
				delete(registry.runeScalars, address)
			}
			registry.mu.Unlock()
			return
		}
		registry.mu.Unlock()
	}
	if !tainted {
		return
	}
	registry := lifecycle.currentRegistry()
	registry.mu.Lock()
	if byteScalar {
		registry.byteScalars[address] = struct{}{}
	} else {
		registry.runeScalars[address] = struct{}{}
	}
	registry.mu.Unlock()
}

func (lifecycle *registryLifecycle) releaseByteScalar(address unsafe.Pointer) {
	for _, registry := range lifecycle.snapshot() {
		registry.mu.Lock()
		delete(registry.byteScalars, address)
		registry.mu.Unlock()
	}
}

func (lifecycle *registryLifecycle) releaseRuneScalar(address unsafe.Pointer) {
	for _, registry := range lifecycle.snapshot() {
		registry.mu.Lock()
		delete(registry.runeScalars, address)
		registry.mu.Unlock()
	}
}
