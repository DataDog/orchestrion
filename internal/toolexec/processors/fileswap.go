// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package processors

import (
	"log"

	"github.com/datadog/orchestrion/internal/toolexec/proxy"
)

type GoFileSwapper struct {
	// Key: file to replace
	// Value: file to replace with
	swapMap map[string]string
}

func NewGoFileSwapper(swapMap map[string]string) GoFileSwapper {
	return GoFileSwapper{
		swapMap: swapMap,
	}
}

func (s *GoFileSwapper) ProcessCompile(cmd *proxy.CompileCommand) {
	log.Printf("[%s] Replacing Go files\n", cmd.Stage())

	for old, new := range s.swapMap {
		if err := cmd.ReplaceParam(old, new); err != nil {
			log.Printf("couldn't replace param: %v\n", err)
		} else {
			log.Printf("====> Replacing %s by %s\n", old, new)
		}
	}
}
