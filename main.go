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

	"github.com/datadog/orchestrion/internal/cmd"
	"github.com/datadog/orchestrion/internal/log"
	"github.com/datadog/orchestrion/internal/version"
	"github.com/urfave/cli/v2"
)

const (
	envVarOrchestrionLogFile  = "ORCHESTRION_LOG_FILE"
	envVarOrchestrionLogLevel = "ORCHESTRION_LOG_LEVEL"
	envVarToolexecImportPath  = "TOOLEXEC_IMPORTPATH"
)

func main() {
	// Setup the logger
	log.SetContext("ORCHESTRION", version.Tag)
	log.SetContext("PID", strconv.FormatInt(int64(os.Getpid()), 10))
	if val := os.Getenv(envVarToolexecImportPath); val != "" {
		log.SetContext(envVarToolexecImportPath, val)
	}
	defer log.Close()

	// Setup the CLI application
	app := cli.App{
		Name:        "orchestrion",
		Usage:       "Automatic compile-time instrumentation of Go code",
		Description: "Orchestrion automatically adds instrumentation to Go applications at compile-time by interfacing with the standard Go toolchain using the -toolexec mechanism to re-write source code before it is passed to the compiler.\n\nFor more information, visit https://datadoghq.dev/orchestrion",
		Copyright:   "2023-present Datadog, Inc.",
		HideVersion: true,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Category: "Logging",
				Name:     "log-level",
				EnvVars:  []string{envVarOrchestrionLogLevel},
				Usage:    "Set the log level (NONE, OFF, ERROR, WARN, INFO, DEBUG, TRACE)",
				Value:    "NONE",
				Action:   actionSetLogLevel,
			},
			&cli.StringFlag{
				Category: "Logging",
				Name:     "log-file",
				EnvVars:  []string{envVarOrchestrionLogFile},
				Usage:    "Send logging output to a file instead of STDERR. Unless --log-level is also specified, the default log level changed to WARN.",
				Action:   actionSetLogFile,
			},
		},
		Commands: []*cli.Command{
			cmd.Go,
			cmd.Pin,
			cmd.Toolexec,
			cmd.Version,
		},
	}
	// Run the CLI application
	if err := app.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(-1)
	}
}

func printUsage(cmd string) {
	commands := []string{
		"go",
		"help",
		"toolexec",
		"version",
		"pin",
		"warmup",
	}
	fmt.Printf("Usage:\n    %s <command> [arguments]\n\n", cmd)
	fmt.Println("Available commands:")
	for _, cmd := range commands {
		fmt.Printf("    %s\n", cmd)
	}
}

var logLevelSet bool

func actionSetLogLevel(_ *cli.Context, level string) error {
	if level, valid := log.LevelNamed(level); valid {
		logLevelSet = true
		log.SetLevel(level)
		return nil
	}
	return fmt.Errorf("invalid log level specified: %q", level)
}

func actionSetLogFile(_ *cli.Context, path string) error {
	if !filepath.IsAbs(path) {
		if wd, err := os.Getwd(); err == nil {
			path = filepath.Join(wd, path)
			os.Setenv(envVarOrchestrionLogFile, path)
		}
	}
	filename := os.Expand(path, func(name string) string {
		switch name {
		case "PID":
			log.SetContext("PID", "")
			return strconv.FormatInt(int64(os.Getpid()), 10)
		default:
			return fmt.Sprintf("$%s", name)
		}
	})
	if err := os.MkdirAll(filepath.Dir(filename), 0o755); err != nil {
		return err
	}
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	log.SetOutput(file)

	if !logLevelSet {
		log.SetLevel(log.LevelWarn)
	}

	return nil
}
