// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package goenv

import "os/exec"

var goBinPath string

// GoBinPath returns the resolved path to the `go` command's binary. The result is cached to avoid
// looking it up multiple times. If the lookup fails, the error is returned and the result is not
// cached.
func GoBinPath() (string, error) {
	if goBinPath == "" {
		goBin, err := exec.LookPath("go")
		if err != nil {
			return "", err
		}
		goBinPath = goBin
	}
	return goBinPath, nil
}
