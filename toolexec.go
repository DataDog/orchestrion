// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"fmt"
	"github.com/datadog/orchestrion/internal/config"
	"github.com/datadog/orchestrion/internal/instrument"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func runToolexecMode(conf config.Config, output func(fullName string, out io.Reader)) error {
	tool, args := os.Args[2], os.Args[3:]
	toolName := filepath.Base(tool)
	if len(args) > 0 && args[0] == "-V=full" {
		// We can't alter the version output.
	} else {
		if toolName == "compile" {
			tmpDir, err := os.MkdirTemp("", "orchestrion")
			if err != nil {
				return err
			}
			defer os.RemoveAll(tmpDir)
			newArgs := make([]string, 0, len(args))
			for _, v := range args {
				if strings.HasSuffix(v, ".go") {
					fullPath, err := filepath.Abs(v)

					if err != nil {
						return fmt.Errorf("Sanitizing path (%s) failed: %v\n", v, err)
					}
					file, err := os.Open(v)
					if err != nil {
						return fmt.Errorf("error opening file: %w", err)
					}
					out, err := instrument.InstrumentFile(fullPath, file, conf)
					file.Close()
					if err != nil {
						return fmt.Errorf("error scanning file %s: %w", fullPath, err)
					}
					newFileName := tmpDir + string(os.PathSeparator) + filepath.Base(fullPath)
					if out != nil {
						output(newFileName, out)
					}
					newArgs = append(newArgs, newFileName)
				} else {
					newArgs = append(newArgs, v)
				}
			}
			args = newArgs
		}
	}
	// Simply run the tool.
	cmd := exec.Command(tool, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}
