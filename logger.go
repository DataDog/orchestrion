// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/datadog/orchestrion/internal/log"
	"github.com/datadog/orchestrion/internal/version"
)

const (
	envVarOrchestrionLogFile  = "ORCHESTRION_LOG_FILE"
	envVarOrchestrionLogLevel = "ORCHESTRION_LOG_LEVEL"
	envVarToolexecImportPath  = "TOOLEXEC_IMPORTPATH"
)

func init() {
	var omitPidContext bool
	if logFile := os.Getenv(envVarOrchestrionLogFile); logFile != "" {
		if !filepath.IsAbs(logFile) {
			// If the path is not absolute, make it absolute w/r/t the current working directory.
			if wd, err := os.Getwd(); err == nil {
				logFile = filepath.Join(wd, logFile)
				// Update the environment variable to reflect the absolute path, so
				// child processes don't have to go through this ordeal again, and so
				// they use the same file even if they have a different current working
				// directory.
				os.Setenv(envVarOrchestrionLogFile, logFile)
			}
		}

		filename := os.Expand(logFile, func(name string) string {
			switch name {
			case "PID":
				omitPidContext = true
				return strconv.FormatInt(int64(os.Getpid()), 10)
			default:
				return "$" + name
			}
		})

		if dir := filepath.Dir(filename); dir != "" {
			// Try to create the parent directory, but ignore errors, if any.
			_ = os.MkdirAll(dir, 0o755)
		}

		file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			panic(fmt.Errorf("unable to open log file %q: %w", filename, err))
		}
		syscall.CloseOnExec(int(file.Fd())) // Don't pass this FD to child processes, it's not useful.
		log.SetLevel(log.LevelWarn)
		log.SetOutput(file)
	}

	if !omitPidContext {
		log.SetContext("PID", strconv.FormatInt(int64(os.Getpid()), 10))
	}
	log.SetContext("ORCHESTRION", version.Tag)
	if ip := os.Getenv(envVarToolexecImportPath); ip != "" {
		log.SetContext(envVarToolexecImportPath, ip)
	}

	if logLevel := os.Getenv(envVarOrchestrionLogLevel); logLevel != "" {
		if level, found := log.LevelNamed(logLevel); !found {
			log.Warnf("invalid log level name %q", logLevel)
		} else {
			log.SetLevel(level)
		}
	}
}
