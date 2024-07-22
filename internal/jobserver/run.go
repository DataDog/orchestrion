// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package jobserver

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/datadog/orchestrion/internal/filelock"
	"github.com/datadog/orchestrion/internal/jobserver/client"
	"github.com/datadog/orchestrion/internal/log"
	"github.com/fsnotify/fsnotify"
)

// Run executes the job server as a standalone process, using the provided arguments.
func Run(args []string) {
	opts := &Options{
		ServerName:        "github.com/datadog/orchestrion/cmd/server",
		InactivityTimeout: time.Minute,
	}
	var urlFile string

	flagSet := flag.NewFlagSet("orchestrion server", flag.ExitOnError)
	flagSet.IntVar(&opts.Port, "port", -1, "Port to listen on. If not set, a random available port will be used.")
	flagSet.Var(durationFlag{&opts.InactivityTimeout}, "inactivity-timeout", "Maximum amount of time to wait for new clients before shutting down.")
	flagSet.BoolVar(&opts.EnableLogging, "enable-logging", false, "Enable NATS server logging.")
	flagSet.StringVar(&urlFile, "url-file", "", "File to write the server's URL to once it is ready to accept connections. If the file already exists and refers to a working server, the server will exit with status 2.")
	flagSet.Parse(args)

	var mu *filelock.Mutex
	if urlFile != "" {
		mu = filelock.MutexAt(urlFile)
		if err := mu.RLock(); err != nil {
			fmt.Fprintf(os.Stderr, "Unable to lock %q: %v\n", urlFile, err)
			os.Exit(1)
		}

		// If the file already contains an URL, check if it has a running server...
		if url, err := os.ReadFile(urlFile); err == nil && len(url) > 0 {
			url := string(url)
			conn, err := client.Connect(url)
			if err == nil {
				log.Infof("Job server is already running at %q\n", url)
				conn.Close()
				os.Exit(2)
			}
		}

		// Upgrade to write-lock
		if err := mu.Lock(); err != nil {
			fmt.Fprintf(os.Stderr, "Unable to upgrade lock %q: %v\n", urlFile, err)
			os.Exit(1)
		}

		// At this stage, we're actually trying to start, so we'll clean up the file after ourselves... First off, we'll do
		// this if we receive an interrupt signal (Control+C); noting that the NATS server will attempt a graceful shutdown
		// on its own here...
		sigChan := make(chan os.Signal, 1)
		defer close(sigChan)
		go func() {
			for range sigChan {
				os.Remove(urlFile)
			}
		}()
		signal.Notify(sigChan, os.Interrupt)
		// Also on normal exit...
		defer os.Remove(urlFile)
	}

	// Start the server for real...
	server, err := New(opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start NATS server: %v\n", err)
		os.Exit(1)
	}

	if urlFile != "" {
		// Write the server's URL to the file...
		if err := os.WriteFile(urlFile, []byte(server.ClientURL()), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to write url file at %q: %v\n", urlFile, err)
			os.Exit(1)
		}

		// Unlock the url file if it was locked...
		if mu != nil {
			if err := mu.Unlock(); err != nil {
				fmt.Fprintf(os.Stderr, "Unable to release lock %q: %v\n", urlFile, err)
				os.Exit(1)
			}
		}

		// Watch for removal of the url file, and shut down the server if that happens...
		watcher, err := fsnotify.NewWatcher()
		if err == nil {
			defer watcher.Close()
			go func() {
				for {
					select {
					case event, ok := <-watcher.Events:
						if !ok {
							return
						}
						if event.Has(fsnotify.Remove) {
							log.Tracef("URL file %q was removed; shutting down...\n", urlFile)
							server.Shutdown()
						}
					case _, ok := <-watcher.Errors:
						if !ok {
							return
						}
					}
				}
			}()
			watcher.Add(urlFile)
		} else {
			log.Warnf("Unable to watch %q for removal: %v\n", urlFile, err)
		}
	}

	// Finally, wait for the server to have shut down...
	server.WaitForShutdown()
}

type durationFlag struct {
	*time.Duration
}

func (f durationFlag) Set(s string) error {
	d, err := time.ParseDuration(s)
	if err != nil {
		return err
	}
	*f.Duration = d
	return nil
}
