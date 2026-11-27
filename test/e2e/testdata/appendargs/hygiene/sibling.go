// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package hygiene

// The declarations below are package-scope names that occupy alias candidates
// for the imports the rewrite needs (H24): Go rejects a file-scope import name
// that equals a package-scope identifier, and the caller values must stay
// intact through the weaving.

var (
	slicesVar = 1 // package-scope "slices"
	optsVar   = 2 // package-scope "opts"
	genSlices = 3 // package-scope "__orchestrion_slices"
	genLib    = 4 // package-scope "__orchestrion_lib"
	appendVar = 5 // package-scope "append"
)
