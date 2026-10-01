// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

//go:build !windows

package files

import "os"

func openLog(name string, truncate bool) (*os.File, error) {
	flag := os.O_CREATE | os.O_WRONLY | os.O_APPEND
	if truncate {
		flag = os.O_CREATE | os.O_WRONLY | os.O_TRUNC
	}
	return os.OpenFile(name, flag, 0o644)
}
