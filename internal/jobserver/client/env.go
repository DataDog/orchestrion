// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package client

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/datadog/orchestrion/internal/filelock"
	"github.com/datadog/orchestrion/internal/log"
)

const (
	ENV_VAR_JOBSERVER_URL = "ORCHESTRION_JOBSERVER_URL"
	urlFileName           = ".orchestrion-jobserver"
)

var (
	client *Client

	ErrNoServerAvailable = errors.New("no job server is available")
)

// FromEnvironment returns a client connected to the current environment's
// job server, using the following process:
//   - If the ORCHESTRION_JOBSERVER_URL environment variable is set, a client
//     connected to this URL is returned.
//   - Otherwise, if workDir is not empty, a server will be identified based on
//     a `.orchestrion-jobserver` file; or a new server will be started using
//     that url file, and a connection will be established to it. The started
//     job server will automatically shut itself down once it no longer has any
//     active client for a period of time.
//   - Otherwise, the ErrNoServerAvailable error is returned.
func FromEnvironment(workDir string) (*Client, error) {
	if client == nil {
		if url := os.Getenv(ENV_VAR_JOBSERVER_URL); url != "" {
			log.Debugf("Connecting to job server at %q (from %s)\n", url, ENV_VAR_JOBSERVER_URL)
			if c, err := Connect(url); err != nil {
				return nil, err
			} else {
				client = c
			}
		} else if workDir != "" {
			log.Debugf("Connecting to job server rooted in %q\n", workDir)
			urlFilePath := filepath.Join(workDir, urlFileName)

			bin, err := os.Executable()
			if err != nil {
				bin = os.Args[0]
			}

			// Try to start a server. The server process is idempotent if the `-url-file` flag is used, so we do not check the
			// command's exit status, because another process might act as our server down the line.
			cmd := exec.Command(bin, "server", "-inactivity-timeout=15m", fmt.Sprintf("-url-file=%s", urlFilePath))
			cmd.SysProcAttr = &sysProcAttrDaemon                   // Make sure go doesn't wait for this to exit...
			cmd.Env = append(os.Environ(), "TOOLEXEC_IMPORTPATH=") // Suppress the TOOLEXEC_IMPORTPATH variable if it's set.
			cmd.Stdin = nil                                        // Connect to `os.DevNull`
			cmd.Stderr, _ = os.Create(urlFilePath + ".stderr.log")
			cmd.Stdout, _ = os.Create(urlFilePath + ".stdout.log")
			if err := cmd.Start(); err != nil {
				return nil, err
			}

			// Wait for the URL file to exist...
			startTime := time.Now()
			timeout := 5 * time.Second
			for {
				if c, url, err := clientFromUrlFile(urlFilePath); err == nil {
					client = c
					// Set it in the current environment so that child processes don't have to go through the same dance again.
					os.Setenv(ENV_VAR_JOBSERVER_URL, url)
					break
				} else if cmd.ProcessState == nil && time.Since(startTime) <= timeout {
					log.Tracef("Job server still not ready in %q...\n", urlFilePath)
					time.Sleep(150 * time.Millisecond)
				} else {
					// Attempt to kill the process if it hasn't died by itself...
					_ = cmd.Process.Kill()
					return nil, err
				}
			}

			// Detach the process, so it survives this one if needed...
			cmd.Process.Release()
		} else {
			log.Debugf("Unable to connect to relevant job server (no environment, no work tree)...\n")
			return nil, ErrNoServerAvailable
		}
	}
	return client, nil
}

func clientFromUrlFile(path string) (*Client, string, error) {
	mu := filelock.MutexAt(path)
	if err := mu.RLock(); err != nil {
		return nil, "", err
	}
	defer func() {
		if err := mu.Unlock(); err != nil {
			log.Warnf("Failed to unlock %q: %v\n", path, err)
		}
	}()

	urlBytes, err := os.ReadFile(path)
	if err != nil {
		return nil, "", err
	}

	if len(urlBytes) == 0 {
		return nil, "", errors.New("blank URL file")
	}

	url := string(urlBytes)
	client, err := Connect(url)
	return client, url, err
}
