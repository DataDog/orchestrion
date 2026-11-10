// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package cgoline

import "example.com/lineinfo/lib"

func OneLiner() (lib.Pos, lib.Pos) { return lib.Caller(), lib.Target() } // mark:oneliner

func MultiLine(m map[string]lib.Pos) {
	m["multi-line"] = lib.Caller()   // mark:multi-line
	m["multi-target"] = lib.Target() // mark:multi-target
}
