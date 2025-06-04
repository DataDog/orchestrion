// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package aspect

import (
	"strings"
)

// weavingSpecialCase defines special behavior to be applied to certain package
// paths. They are evaluated in order, and the first matching override is
// applied, stopping evaluation of any further overrides.
var weavingSpecialCase = []specialCase{
	// Weaving inside of orchestrion packages themselves
	{path: "github.com/DataDog/orchestrion/runtime", prefix: true, behavior: NoOverride},
	{path: "github.com/DataDog/orchestrion", prefix: true, behavior: NeverWeave},
	// V1 of the Datadog Go tracer library
	{path: "gopkg.in/DataDog/dd-trace-go.v1", prefix: true, behavior: WeaveTracerInternal},
	// V2 of the Datadog Go tracer library
	{path: "github.com/DataDog/dd-trace-go/internal/orchestrion/_integration", prefix: true, behavior: NoOverride},    // The dd-trace-go integration test suite
	{path: "github.com/DataDog/dd-trace-go/v2/internal/orchestrion/_integration", prefix: true, behavior: NoOverride}, // The dd-trace-go integration test suite
	{path: "github.com/DataDog/dd-trace-go", prefix: true, behavior: WeaveTracerInternal},
	// Misc. other Datadog packages that can cause circular weaving to happen
	{path: "github.com/DataDog/go-tuf/client", prefix: false, behavior: NeverWeave},
}

type (
	specialCase struct {
		path     string
		prefix   bool
		behavior BehaviorOverride
	}

	BehaviorOverride int
)

const (
	// NoOverride does not change the injector behavior, but prevents further
	// rules from being applied.
	NoOverride BehaviorOverride = iota
	// NeverWeave completely disables injecting into the designated package
	// path(s).
	NeverWeave
	// WeaveTracerInternal limits weaving to only aspects that have the
	// `tracer-internal` flag set.
	WeaveTracerInternal
)

// Matches returns true if the importPath is matched by this special case
func (sc *specialCase) matches(importPath string) bool {
	return importPath == sc.path || sc.prefix && strings.HasPrefix(importPath, sc.path+"/")
}

// FindBehaviorOverride checks the import path against the weaver special cases and returns a potential special case
func FindBehaviorOverride(importPath string) (BehaviorOverride, bool) {
	for _, sc := range weavingSpecialCase {
		if sc.matches(importPath) {
			return sc.behavior, true
		}
	}
	return NoOverride, false
}
