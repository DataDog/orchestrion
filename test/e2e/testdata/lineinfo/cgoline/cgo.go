// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

// Package cgoline records runtime positions in a cgo file and a plain file.
package cgoline

/*
static int twice(int x) { return 2 * x; }
*/
import "C"

import "example.com/lineinfo/lib"

func Twice(x int) int { return int(C.twice(C.int(x))) }

func CgoLines(m map[string]lib.Pos) {
	m["cgo-before"] = lib.Caller() // mark:cgo-before
	m["cgo-target"] = lib.Target() // mark:cgo-target
	m["cgo-after"] = lib.Caller()  // mark:cgo-after
}
