// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

// Package victim is the instrumented package. The `orchestrion.yml` aspect makes
// its Work function depend on the tracer package, which depends on this one.
package victim

// Name is used by the tracer package, so that it depends on this one.
const Name = "victim"

func Work() string {
	return Name
}
