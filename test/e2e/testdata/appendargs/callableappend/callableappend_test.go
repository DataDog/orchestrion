// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package callableappend

import (
	"testing"

	"example.com/appendargs/check"
	"example.com/appendargs/lib"
)

// TestHygieneCallableAppend is H07: the caller's append is a callable function
// with a compatible signature and a counter; the woven call must not call it
// even where a call would compile, and the result must stay correct.
func TestHygieneCallableAppend(t *testing.T) {
	lib.Record.Reset()
	CallableAppend([]lib.Option{lib.Named("x")})
	check.LastNames(t, "x", "traced")
	check.True(t, AppendCalls() == 0, "the woven code must not call the caller's append")
}
