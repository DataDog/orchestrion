// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

// Package tracer stands in for an integration's contrib package: it is injected
// into victim.Work, and it depends on the victim package itself.
package tracer

import "example.com/compileloop/victim"

func Trace() {
	_ = victim.Name
}
