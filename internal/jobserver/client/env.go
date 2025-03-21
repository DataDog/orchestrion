// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package client

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
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
//
// The returned client is re-used, so callers should NOT call [Client.Close] on
// it.
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

	exitChan := make(chan error)
	go func() {
		defer close(exitChan)
		exitChan <- cmd.Wait()
	}()

	// Wait for the URL file to exist...
	log.Trace().
		Str("url-file", urlFilePath).
		Stringer("timeout", jobserverStartTimeout).
		Msg("Waiting for job server to come online...")
	ctx, cancel := context.WithTimeout(context.Background(), jobserverStartTimeout)
	defer cancel()
	client, err := waitForURLFile(ctx, urlFilePath, cmd, exitChan)
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

	file := filelock.MutexAt(path)
	if err := file.RLock(ctx); err != nil {
		return nil, "", err
	}
	defer func() {
		if err := file.Unlock(ctx); err != nil {
			log.Warn().Str("url-file", path).Err(err).Msg("Failed to unlock file")
		}
	}()

	urlBytes, err := io.ReadAll(file)
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

func waitForURLFile(ctx context.Context, path string, cmd *exec.Cmd, exitChan <-chan error) (*Client, error) {
	const retryDelay = 150 * time.Millisecond
	var (
		log   = zerolog.Ctx(ctx)
		retry *time.Timer
	)

	for {
		// First, try to connect to the client from the URL file.
		c, url, err := clientFromURLFile(ctx, path)
		if err == nil {
			// There was no error, so we are good to go!
			client = c
			// Set it in the current environment so that child processes don't have to go through the same dance again.
			_ = os.Setenv(EnvVarJobserverURL, url)
			return c, nil
		}
		if url != "" {
			log.Error().Err(err).
				Str("url-file", path).
				Str("url", url).
				Msg("Failed to connect to job server at specified URL")
			return nil, err
		}

		log.Trace().Err(err).Str("url-file", path).Msg("Job server still not ready...")
		if retry == nil {
			retry = time.NewTimer(retryDelay)
			//revive:disable:defer This happens only once in the loop, and is to avoid starting a timer we don't use
			defer retry.Stop()
			//revive:enable:defer
		} else {
			retry.Reset(retryDelay)
		}

		select {
		case <-ctx.Done(): // If the context is Done, we should not be waiting any longer...
			ctxErr := ctx.Err()
			if ctxErr == nil {
				ctxErr = errors.New("wait context has expired")
			}
			log.Warn().
				Err(ctxErr).
				Str("url-file", path).
				Msg("Context aborted while waiting for url-file")
			// Attempt to kill the process if it hasn't died by itself...
			return nil, errors.Join(err, ctxErr, cmd.Process.Kill())

		case exitErr, ok := <-exitChan: // If the process has exited, there is no use to waiting any longer...
			if exitErr != nil {
				log.Warn().
					Err(exitErr).
					Stringer("state", cmd.ProcessState).
					Str("url-file", path).
					Msg("Job server process has exited")
				return nil, errors.Join(err, exitErr, fmt.Errorf("job server process has failed: %v", cmd.ProcessState))
			}
			if ok {
				// The job server exits with status 0 if another process has written to the URL file; in
				// which case we should be able to connect to it on the next attempt!
				log.Info().
					Stringer("state", cmd.ProcessState).
					Str("url-file", path).
					Msg("Job server process exited with status 0 (another process is serving)")
			}

		case <-retry.C:
			// The retry timer has elapsed, we shall try again!
			continue
		}
	}
}

var jobserverStartTimeout = 5 * time.Second

func init() {
	const envVarName = "ORCHESTRION_JOB_SERVER_START_TIMEOUT_SECONDS"
	val := os.Getenv(envVarName)
	if val == "" {
		return
	}

	sec, err := strconv.Atoi(val)
	if err != nil {
		_, _ = fmt.Fprintf(
			os.Stderr,
			"Warning: unable to parse value of "+envVarName+"=%q due to %v, will use default value of %s instead\n",
			val,
			err,
			jobserverStartTimeout,
		)
		return
	}

	jobserverStartTimeout = time.Duration(sec) * time.Second
}
