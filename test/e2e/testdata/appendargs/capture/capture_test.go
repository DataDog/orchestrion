// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package capture

import (
	"testing"

	"example.com/appendargs/check"
	"example.com/appendargs/lib"
)

// TestCaptureParameter is H04: the injected value reads the caller's opts
// parameter, not a generated binding or the spread slice.
func TestCaptureParameter(t *testing.T) {
	lib.Record.Reset()
	Parameter("user-value", []lib.Option{lib.Named("x")})
	check.LastNames(t, "x", "user-value")
}

// TestCaptureLocalBlock is H05: the injected value reads the innermost caller
// binding named opts.
func TestCaptureLocalBlock(t *testing.T) {
	lib.Record.Reset()
	LocalBlock("outer-value", []lib.Option{lib.Named("x")})
	check.LastNames(t, "x", "inner-value")
}

// TestCaptureLocalOnly checks a local without any parameter of the same name.
func TestCaptureLocalOnly(t *testing.T) {
	lib.Record.Reset()
	LocalOnly([]lib.Option{lib.Named("x")})
	check.LastNames(t, "x", "local-value")
}
