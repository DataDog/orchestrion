// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package cmd

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/DataDog/orchestrion/internal/filelock"
	"github.com/DataDog/orchestrion/internal/jobserver"
	"github.com/DataDog/orchestrion/internal/jobserver/client"
	"github.com/DataDog/orchestrion/internal/log"
	"github.com/fsnotify/fsnotify"

	"github.com/urfave/cli/v2"
)

var Server = &cli.Command{
	Name:        "server",
	Usage:       "Start an Objectsrion job server.",
	Description: "The job server is used to remove duplicated processing that can occur when instrumenting large applications, due to how Orchestrion injects new dependencies that the go toolchain was initially not aware of.\n\nUsers do not normally need to use this command directly, as Orchestrion automatically manages servers during runtime.",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "url-file",
			Usage: "Write a file containing the ClientURL for this server once it is ready to accept connections. The server automatically shuts down when the URL file is deleted.",
		},
		&cli.IntFlag{
			Name:        "port",
			Usage:       "Choose a port to listen on",
			Value:       -1,
			DefaultText: "random",
		},
		&cli.DurationFlag{
			Name:  "inactivity-timeout",
			Usage: "Automatically shut down after a period without any connected client",
			Value: time.Minute,
		},
		&cli.BoolFlag{
			Name:  "nats-logging",
			Usage: "Enable NATS server logging",
		},
	},
	Hidden: true,
	Action: func(c *cli.Context) error {
		opts := jobserver.Options{
			ServerName:        "github.com/DataDog/orchestrion server",
			Port:              c.Int("port"),
			InactivityTimeout: c.Duration("inactivity-timeout"),
			EnableLogging:     c.Bool("nats-logging"),
		}

		if urlFile := c.String("url-file"); urlFile != "" {
			return startWithURLFile(&opts, urlFile)
		}
		_, err := start(&opts, true)
		return err
	},
}

// start starts a new job server, and waits for it to have completely shut down if `wait` is true.
// When `wait` is true, the server is always returned as `nil`.
func start(opts *jobserver.Options, wait bool) (*jobserver.Server, cli.ExitCoder) {
	server, err := jobserver.New(opts)
	if err != nil {
		return nil, cli.Exit(fmt.Errorf("failed to start job server: %w", err), 1)
	}

	if wait {
		server.WaitForShutdown()
		return nil, nil
	}

	return server, nil
}

// startWithURLFile starts a new job server using the provided URL file (unless the file contains the URL to a still
// running server), and waits for it to have completely shut down.
func startWithURLFile(opts *jobserver.Options, urlFile string) cli.ExitCoder {
	mu := filelock.MutexAt(urlFile)
	if err := mu.RLock(); err != nil {
		return cli.Exit(fmt.Errorf("failed to acquire read lock on %q: %w", urlFile, err), 1)
	}

	// Check if there is already a server running...
	if url, err := hasURLToRunningServer(urlFile); err != nil {
		return cli.Exit(err, 1)
	} else if url != "" {
		return cli.Exit(fmt.Sprintf("A server is already listening on %q", url), 2)
	}

	// No existing server, so now we're actually going to try starting our own
	if err := mu.Lock(); err != nil {
		return cli.Exit(fmt.Errorf("failed to upgrade to write lock on %q: %w", urlFile, err), 1)
	}

	// Check again whether there is a running server; as a concurrent process might have acquired the write lock first.
	if url, err := hasURLToRunningServer(urlFile); err != nil {
		return cli.Exit(err, 1)
	} else if url != "" {
		return cli.Exit(fmt.Sprintf("A server is already listening on %q", url), 2)
	}

	// This process "owns" the URL file, so it'll try had to remove it when it terminates...
	cancelDeleteOnInterrupt := deleteOnInterrupt(urlFile)
	defer cancelDeleteOnInterrupt()
	defer os.Remove(urlFile)

	// Start the server normally...
	server, err := start(opts, false)
	if err != nil {
		return err
	}

	// Write the ClientURL into the urlFile
	if err := os.WriteFile(urlFile, []byte(server.ClientURL()), 0o644); err != nil {
		return cli.Exit(fmt.Errorf("failed to write URL file at %q: %w", urlFile, err), 1)
	}
	// Release the URL File lock
	if err := mu.Unlock(); err != nil {
		// Shut the server down, as we won't actually be returning it...
		server.Shutdown()
		return cli.Exit(fmt.Errorf("failed to release lock on %q: %w", urlFile, err), 1)
	}

	// Try to watch for removal of the URL file, so we can shut down the server eagerly when that happens.
	cancelShutdownOnRemove := shutdownOnRemove(server, urlFile)
	defer cancelShutdownOnRemove()

	server.WaitForShutdown()
	return nil
}

// deleteOnInterrupt attempts to deletes the provided file when an interrupt signal is received. It returns a
// cancellation function that can be used to uninstall the signal handler.
func deleteOnInterrupt(path string) func() {
	sigChan := make(chan os.Signal, 1)
	cancel := func() {
		signal.Stop(sigChan)
		close(sigChan)
	}

	go func() {
		_, closed := <-sigChan
		if closed {
			return
		}
		defer cancel()
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			log.Warnf("Failed to remove %q: %v\n", path, err)
		}
	}()

	signal.Notify(sigChan, os.Interrupt)

	return cancel
}

// hasURLToRunningServer checks whether the provided URL file contains the URL to a running server,
// by trying to connect to it. If that is the case, it returns the URL to the running server.
func hasURLToRunningServer(urlFile string) (string, error) {
	urlData, err := os.ReadFile(urlFile)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("failed to read URL file at %q: %w", urlFile, err)
	}
	if len(urlData) == 0 {
		return "", nil
	}

	url := string(urlData)
	conn, err := client.Connect(url)
	if err != nil {
		return "", nil
	}
	conn.Close()
	return url, nil
}

// shutdownOnRemove shuts the server down when the designated file is removed. It returns a cancellation function that
// can be used to cancel the file watcher. Since fsnotify support is highly dependent on platform/kernel support, this
// function ignores any error and emits WARN log entries describing the problem.
func shutdownOnRemove(server *jobserver.Server, urlFile string) func() error {
	// noCancel is returned when there is nothing to cancel...
	noCancel := func() error { return nil }

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Warnf("Failed to create fsnotify watcher: %v\n", err)
		log.Warnf("The server will not automatically shut down when the URL file (%q) is removed; only when it reaches the configured inactivity timeout.", urlFile)
		return noCancel
	}
	cancel := watcher.Close

	if err := watcher.Add(urlFile); err != nil {
		defer cancel()
		log.Warnf("Failed to watch URL file at %q: %v\n", urlFile, err)
		log.Warnf("The server will not automatically shut down when the URL file (%q) is removed; only when it reaches the configured inactivity timeout.", urlFile)
		return noCancel
	}

	go func(events <-chan fsnotify.Event, errors <-chan error) {
		for {
			select {
			case event, ok := <-events:
				if !ok {
					return
				}
				if event.Has(fsnotify.Remove) {
					log.Tracef("URL file at %q was removed; shutting down...\n", event.Name)
					server.Shutdown()
				}
			case err, ok := <-errors:
				if !ok {
					return
				}
				log.Warnf("File watcher produced an error: %v\n", err)
			}
		}
	}(watcher.Events, watcher.Errors)

	return cancel
}
