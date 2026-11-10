// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

// Package lib reports call-site positions as the runtime sees them.
package lib

import "runtime"

// Pos is the file and line of a call site, as the runtime reports it.
type Pos struct {
	File string
	Line int
}

// Caller returns the position of its call site. The aspect does not touch it.
func Caller() Pos {
	_, file, line, _ := runtime.Caller(1)
	return Pos{File: file, Line: line}
}

// Target returns the position of its call site. The aspect wraps every call, so this
// reports the position of the wrapped call site.
func Target() Pos {
	_, file, line, _ := runtime.Caller(1)
	return Pos{File: file, Line: line}
}

// Wrap is what the aspect wraps around every lib.Target() call.
func Wrap(v Pos) Pos { return v }
