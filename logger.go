// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"fmt"
	"os"
	"path"
	"strconv"

	"github.com/datadog/orchestrion/internal/log"
	"github.com/datadog/orchestrion/internal/version"
)

func init() {
	var omitPidContext bool
	if logFile := os.Getenv("ORCHESTRION_LOG_FILE"); logFile != "" {
		filename := os.Expand(logFile, func(name string) string {
			switch name {
			case "PID":
				omitPidContext = true
				return strconv.FormatInt(int64(os.Getpid()), 10)
			default:
				return "$" + name
			}
		})

		// Try to create the parent directory, but ignore errors, if any.
		_ = os.MkdirAll(path.Dir(filename), 0o755)

		file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			panic(fmt.Errorf("unable to open log file %q: %w", logFile, err))
		}
		log.SetLevel(log.LevelWarn)
		log.SetOutput(file)
	}

	if !omitPidContext {
		log.SetContext("PID", strconv.FormatInt(int64(os.Getpid()), 10))
	}
	log.SetContext("ORCHESTRION", version.Tag)
	if ip := os.Getenv("TOOLEXEC_IMPORTPATH"); ip != "" {
		log.SetContext("TOOLEXEC_IMPORTPATH", ip)
	}

	if logLevel := os.Getenv("ORCHESTRION_LOG_LEVEL"); logLevel != "" {
		if level, found := log.LevelNamed(logLevel); !found {
			log.Warnf("invalid log level name %q", logLevel)
		} else {
			log.SetLevel(level)
		}
	}
}
