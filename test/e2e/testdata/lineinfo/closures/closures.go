// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

// Package closures records runtime positions around immediately invoked,
// deferred, and concurrently started function literals.
package closures

import "example.com/lineinfo/lib"

func Bodies() map[string]lib.Pos {
	m := map[string]lib.Pos{}

	// The channel makes the goroutine's result deterministic: the value is
	// consumed before this function returns.
	ch := make(chan lib.Pos, 1)
	go func() { ch <- lib.Caller() }()                   // mark:go-literal
	defer func() { m["defer-literal"] = lib.Caller() }() // mark:defer-literal

	func() { m["iife"] = lib.Caller() }() // mark:iife

	m["after-iife"] = lib.Caller() // mark:after-iife
	m["target"] = lib.Target()     // mark:target
	m["go-literal"] = <-ch
	return m
}
