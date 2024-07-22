// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package common

import "github.com/nats-io/nats.go"

// Fork returns a function that calls the given callback in a new goroutine.
func Fork(cb func(*nats.Msg)) func(*nats.Msg) {
	return func(msg *nats.Msg) { go cb(msg) }
}
