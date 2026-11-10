// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package capture

import "example.com/appendargs/lib"

// Parameter holds a string parameter named opts, the historical closure
// parameter name of the append-args rewrite. The injected value reads this
// parameter.
func Parameter(opts string, args []lib.Option) {
	lib.SinkCaptureParam(args...)
}

// LocalBlock declares a local opts in a nested block, with a different outer
// opts in scope as well. The injected value must read the inner binding.
func LocalBlock(outer string, args []lib.Option) {
	opts := "outer-value"
	{
		opts := "inner-value"
		lib.SinkCaptureLocal(args...)
		_ = opts
	}
	_ = opts
	_ = outer
}

// LocalOnly has no opts parameter; the injected value binds to the local.
func LocalOnly(args []lib.Option) {
	opts := "local-value"
	lib.SinkCaptureLocal(args...)
	_ = opts
}
