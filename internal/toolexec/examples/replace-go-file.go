// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package examples

import (
	"github.com/datadog/orchestrion/internal/toolexec/proxy"
	"github.com/datadog/orchestrion/internal/toolexec/utils"
)

func ExampleReplaceGoFile() {
	// In a real use case command arguments should be populated by reading from os.Args
	args := []string{"/random/compile", "-trimpath", "randompath", "-p", "random", "-o", "/tmp/randomBuild/_pkg_.a", "-importcfg", "/tmp/random/b002/importcfg", "main.go"}
	cmd, err := proxy.ParseCommand(args)
	utils.ExitIfError(err)
	filesReplacer := goFilesReplacer{files: map[string]string{"main.go": "custom-main.go"}}
	proxy.ProcessCommand(cmd, filesReplacer.ProcessCompile)
}

type goFilesReplacer struct {
	files map[string]string
}

func (i goFilesReplacer) ProcessCompile(cmd *proxy.CompileCommand) error {
	for old, new := range i.files {
		if err := cmd.ReplaceParam(old, new); err != nil {
			return err
		}
	}
	return nil
}
