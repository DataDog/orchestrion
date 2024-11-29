// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package binpath

import (
	"os"
	"path/filepath"
)

var Orchestrion string

func init() {
	var err error
	if Orchestrion, err = os.Executable(); err != nil {
		if Orchestrion, err = filepath.Abs(os.Args[0]); err != nil {
			Orchestrion = os.Args[0]
		}
	}
	Orchestrion = filepath.Clean(Orchestrion)
}
