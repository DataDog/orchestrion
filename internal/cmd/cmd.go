// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package cmd

import (
	"os"
	"path/filepath"
)

var orchestrionBinPath string

func init() {
	var err error
	if orchestrionBinPath, err = os.Executable(); err != nil {
		if orchestrionBinPath, err = filepath.Abs(os.Args[0]); err != nil {
			orchestrionBinPath = os.Args[0]
		}
	}
	orchestrionBinPath = filepath.Clean(orchestrionBinPath)
}
