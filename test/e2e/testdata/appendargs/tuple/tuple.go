// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

// Package tuple holds the sole multi-value call that the advice must reject.
// The package builds un-woven; the woven build fails with the advice error, so
// the end-to-end test asserts the failure instead of running it.
package tuple

import "example.com/appendargs/lib"

// TupleSinkCall spreads two values into the any-typed sink. The advice rejects
// it instead of emitting invalid Go.
func TupleSinkCall() {
	lib.TupleSink(lib.TwoOpts())
}
