// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package proxy

import "errors"

var (
	// ErrSkipCommand is returned by command processors to indicate that the
	// command should not be executed, and instead considered an idempotent
	// success.
	ErrSkipCommand = errors.New("skip command")
)
