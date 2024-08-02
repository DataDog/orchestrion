// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/datadog/orchestrion/internal/filelock"
	"github.com/datadog/orchestrion/internal/jobserver"
	"github.com/datadog/orchestrion/internal/jobserver/client"
	"github.com/datadog/orchestrion/internal/log"
	"github.com/fsnotify/fsnotify"

	"github.com/urfave/cli/v2"
)

var Server = &cli.Command{
	Name:        "server",
	Usage:       "Start an Objectsrion job server.",
	Description: "The job server is used to remove duplicated processing that can occur when isntrumenting large applications, due to how Orchestrion injects new dependencies that the go toolchain was initially not aware of.\n\nUsers do not normally need to use this command directly, as Orchestrion automatically manages servers during runtime.",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "url-file",
			Usage: "Write a file containing the ClientURL for this server once it is ready to accept connections",
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
			ServerName:        "github.com/datadog/orchestrion server",
			Port:              c.Int("port"),
			InactivityTimeout: c.Duration("inactivity-timeout"),
			EnableLogging:     c.Bool("nats-logging"),
		}
		urlFile := c.String("url-file")

		var mu *filelock.Mutex
		if urlFile != "" {
			mu = filelock.MutexAt(urlFile)
			if err := mu.RLock(); err != nil {
				return cli.Exit(fmt.Errorf("unable to acquire read lock on %q: %w", urlFile, err), 1)
			}

			if urlData, err := os.ReadFile(urlFile); err == nil && len(urlData) > 0 {
				url := string(urlData)
				conn, err := client.Connect(url)
				if err == nil {
					defer conn.Close()
					return cli.Exit(fmt.Sprintf("A job server is already available at %q", url), 2)
				}
			}

			// Upgrade to a write lock
			if err := mu.Lock(); err != nil {
				return cli.Exit(fmt.Errorf("unable to acquire write lock on %q: %w", urlFile, err), 1)
			}

			// At this stage, we're actually going to start, and we will clean up the urlFile after ourselces. This includes
			// cases when the server is killed by an interrupt signal (Control+C). Note the NATS server has its own signal
			// handler and will attempt to shut itself down gracefully so we are not doing this here...
			sigChan := make(chan os.Signal, 1)
			defer close(sigChan)
			go func() {
				for range sigChan {
					os.Remove(urlFile)
				}
			}()
			signal.Notify(sigChan, os.Interrupt)
			// We also clean up the file on regular exit
			defer os.Remove(urlFile)
		}

		// Start the server for real now...
		server, err := jobserver.New(&opts)
		if err != nil {
			return cli.Exit(fmt.Errorf("failed to start NATS server: %w", err), 1)
		}

		if urlFile != "" {
			// Now, write the server's URL to the file...
			if err := os.WriteFile(urlFile, []byte(server.ClientURL()), 0o644); err != nil {
				return cli.Exit(fmt.Errorf("failed to write URL file at %q: %w", urlFile, err), 1)
			}

			// Unlock the URL file if it was locked...
			if mu != nil {
				if err := mu.Unlock(); err != nil {
					return cli.Exit(fmt.Errorf("failed to unlock %q: %w", urlFile, err), 1)
				}
			}

			// Watch for removal of the URL file, and shut down the server if/when that happens... Note that if we are unable
			// to watch, we'll leave it entirely up to the inactivity timeout instead...
			watcher, err := fsnotify.NewWatcher()
			if err != nil {
				log.Warnf("Failed to create file watcher: %v\n", err)
			} else {
				defer watcher.Close()
				go shutdownOnRemove(watcher, server)
				watcher.Add(urlFile)
			}
		}

		// Wait indefinitely for the server to shut down...
		server.WaitForShutdown()
		return nil
	},
}

func shutdownOnRemove(watcher *fsnotify.Watcher, server *jobserver.Server) {
	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			if event.Has(fsnotify.Remove) {
				log.Tracef("URL file at %q was removed; shutting down...\n", event.Name)
				server.Shutdown()
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			log.Warnf("File watcher produced an error: %v\n", err)
		}
	}
}
