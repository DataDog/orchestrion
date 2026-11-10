// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

// Package app holds the woven spread call of the go1.17 language fixture.
package app

import "example.com/appendargs-go117/local"

// Woven spreads into the sink; the rewrite compiles through the minimum
// language propagation even though the module stays at go 1.17.
func Woven(xs []local.Opt) {
	local.Sink(xs...)
}
