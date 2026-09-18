// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

// Package dependency stands in for a third-party module the fixture cannot
// actually vendor. It is a local subpackage of this fixture directory, not a
// real external dependency; see cases.json and the Lane B evidence for the
// honest limitation this substitution implies.
package dependency

// Clone returns a freshly allocated copy of s via a []byte round trip, the
// same allocation shape a real dependency's clone helper would use.
func Clone(s string) string {
	b := []byte(s)
	fresh := make([]byte, len(b))
	copy(fresh, b)
	return string(fresh)
}
