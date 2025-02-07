// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package client

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/DataDog/orchestrion/internal/binpath"
	"github.com/DataDog/orchestrion/internal/filelock"
	"github.com/rs/zerolog"
)

const (
	EnvVarJobserverURL = "ORCHESTRION_JOBSERVER_URL"
	urlFileName        = ".orchestrion-jobserver"
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
func FromEnvironment(ctx context.Context, workDir string) (*Client, error) {
	if client != nil {
		return client, nil
	}

	log := zerolog.Ctx(ctx)

	if url := os.Getenv(EnvVarJobserverURL); url != "" {
		log.Debug().Str(EnvVarJobserverURL, url).Msg("Connecting to job server")
		c, err := Connect(url)
		if err != nil {
			return nil, err
		}
		client = c
		return client, nil
	}

	if workDir == "" {
		log.Debug().Msg("Unable to connect to relevant job server (no environment, no work tree)...")
		return nil, ErrNoServerAvailable
	}

	log.Debug().Str("workdir", workDir).Msg("Connecting to job server rooted in working directory")
	urlFilePath := filepath.Join(workDir, urlFileName)

	// Try to start a server. The server process is idempotent if the `-url-file` flag is used, so we do not check the
	// command's exit status, because another process might act as our server down the line.
	cmd := exec.Command(binpath.Orchestrion, "server",
		"-inactivity-timeout=15m",
		fmt.Sprintf("-url-file=%s", urlFilePath),
		fmt.Sprintf("-parent-pid=%d", os.Getpid()),
	)
	cmd.SysProcAttr = &sysProcAttrDaemon                   // Make sure go doesn't wait for this to exit...
	cmd.Env = append(os.Environ(), "TOOLEXEC_IMPORTPATH=") // Suppress the TOOLEXEC_IMPORTPATH variable if it's set.
	cmd.WaitDelay = jobserverStartTimeout
	cmd.Stdin = nil // Connect to `os.DevNull`
	cmd.Stderr, _ = os.Create(urlFilePath + ".stderr.log")
	cmd.Stdout, _ = os.Create(urlFilePath + ".stdout.log")
	log.Trace().
		Strs("args", cmd.Args).
		Msg("Starting deamonized jobserver process...")
	if err := cmd.Start(); err != nil {
		return nil, err
	}

	// Wait for the URL file to exist...
	log.Trace().
		Str("url-file", urlFilePath).
		Stringer("timeout", jobserverStartTimeout).
		Msg("Waiting for job server to come online...")
	ctx, cancel := context.WithTimeout(context.Background(), jobserverStartTimeout)
	defer cancel()
	client, err := waitForURLFile(ctx, urlFilePath, cmd)
	if err != nil {
		err = errors.Join(err, cmd.Process.Kill()) // Kill the process if it's still running...
		log.Warn().
			Err(err).
			Str("url-file", urlFilePath).
			Msg("Job server could not come online, aborting")
		return nil, err
	}
	// Detach the process, so it survives this one if needed...
	if err := cmd.Process.Release(); err != nil {
		log.Warn().Err(err).Msg("Failed to detach from job server process")
	}

	return client, nil
}

func clientFromURLFile(ctx context.Context, path string) (*Client, string, error) {
	log := zerolog.Ctx(ctx)

	mu := filelock.MutexAt(path)
	if err := mu.RLock(); err != nil {
		return nil, "", err
	}
	defer func() {
		if err := mu.Unlock(); err != nil {
			log.Warn().Str("url-file", path).Err(err).Msg("Failed to unlock file")
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
	log.Trace().
		Err(err).
		Str("url-file", path).
		Str("url", url).
		Msg("Connected to job server from URL file")
	return client, url, err
}

func waitForURLFile(ctx context.Context, path string, cmd *exec.Cmd) (*Client, error) {
	log := zerolog.Ctx(ctx)
	for {
		// First, try to connect to the client from the URL file.
		c, url, err := clientFromURLFile(ctx, path)
		if err != nil {
			// Check whether the child process is still alive by sending it signal 0. This returns an
			// error if the process is dead, and does nothing if it's alive.
			if cmd.Process.Signal(syscall.Signal(0)) != nil {
				// The process died, so we can call [exec.Cmd.Wait] on it to clean up associated resources.
				waitErr := cmd.Wait()
				log.Warn().
					Err(err).
					Stringer("state", cmd.ProcessState).
					Str("url-file", path).
					Msg("Job server process has exited")
				return nil, errors.Join(
					err,
					waitErr,
					fmt.Errorf("job server process has exited: %v", cmd.ProcessState),
				)
			}

			// The process has not died, so no we'll check whether the context has been cancelled/expired.
			if ctxErr := ctx.Err(); ctxErr != nil {
				log.Warn().
					Err(ctxErr).
					Str("url-file", path).
					Msg("Context aborted while waiting for url-file")
				// Attempt to kill the process if it hasn't died by itself...
				return nil, errors.Join(err, ctxErr, cmd.Process.Kill())
			}

			// The process has not died, and the context has not expired yet, so we'll wait and retry...
			log.Trace().Err(err).Str("url-file", path).Msg("Job server still not ready...")
			time.Sleep(150 * time.Millisecond)
			continue
		}

		// There was no error, so we are good to go!
		client = c
		// Set it in the current environment so that child processes don't have to go through the same dance again.
		_ = os.Setenv(EnvVarJobserverURL, url)
		return c, nil
	}
}

var jobserverStartTimeout = 5 * time.Second

func init() {
	val := os.Getenv("ORCHESTRION_JOB_SERVER_START_TIMEOUT_SECONDS")
	if val == "" {
		return
	}

	sec, err := strconv.Atoi(val)
	if err != nil {
		// We got an invalid value, we'll silently ignore this because we can't ensure the logger has been initialized yet.
		return
	}

	jobserverStartTimeout = time.Duration(sec) * time.Second
}
