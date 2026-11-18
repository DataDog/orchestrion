// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

// Package noop holds the zero-template advice target of the go1.17 language
// fixture: the no-op advice neither rewrites the call nor raises the required
// language version, so the package stays valid at go 1.17.
package noop

import "example.com/appendargs-go117/local"

// Noop spreads into the zero-template sink.
func Noop(xs []local.Opt) {
	local.SinkZero(xs...)
}

// Control is the positive control for the zero-template no-op: the local.Sink
// aspect is live in this module, so this call weaves and its value appears.
// It proves the no-op assertion tests a woven package, not a dead config.
func Control(xs []local.Opt) {
	local.Sink(xs...)
}
