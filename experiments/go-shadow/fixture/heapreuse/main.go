// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"os"
	"runtime"
	"unsafe"
	"weak"
)

type box struct {
	path string
}

//go:noinline
func taintedBox() (uintptr, weak.Pointer[box]) {
	value := new(box)
	value.path = os.Getenv("TAINT_PATH")
	_, _ = os.Open(value.path)
	address := uintptr(unsafe.Pointer(value))
	pointer := weak.Make(value)
	runtime.KeepAlive(value)
	return address, pointer
}

func main() {
	deadAddress, pointer := taintedBox()
	for cycle := 0; cycle < 10 && pointer.Value() != nil; cycle++ {
		runtime.GC()
	}
	if pointer.Value() != nil {
		panic("tainted heap object was not collected")
	}

	values := make([]*box, 0, 4096)
	for attempt := 0; attempt < 1_000_000; attempt++ {
		value := &box{}
		values = append(values, value)
		if uintptr(unsafe.Pointer(value)) == deadAddress {
			_, _ = os.Open(value.path)
			runtime.KeepAlive(values)
			return
		}
	}
	panic("allocator did not reuse the dead object address")
}
