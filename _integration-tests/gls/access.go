// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package gls

import (
	_ "runtime" // Provides go:linkname targets (if Orchestrion modifies)
	_ "unsafe"  // For go:linkname
)

var (
	//go:linkname __dd_orchestrion_gls_get __dd_orchestrion_gls_get
	__dd_orchestrion_gls_get func() any

	//go:linkname __dd_orchestrion_gls_set __dd_orchestrion_gls_set
	__dd_orchestrion_gls_set func(any)

	get = func() any { return nil }
	set = func(any) {}
)

func init() {
	if __dd_orchestrion_gls_get != nil {
		get = __dd_orchestrion_gls_get
	}
	if __dd_orchestrion_gls_set != nil {
		set = __dd_orchestrion_gls_set
	}
}
