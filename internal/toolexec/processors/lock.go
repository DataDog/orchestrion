// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package processors

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/rogpeppe/go-internal/lockedfile"
)

func waitUntilNonEmpty(filename string, timeout time.Duration) ([]byte, error) {
	log.Printf("Acquiring %q for reading...\n", filename)
	start := time.Now()
	for {
		if time.Since(start) > timeout {
			return nil, fmt.Errorf("timed out waiting to acquire lock %q", filename)
		}

		lockedFile, err := lockedfile.Open(filename)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				log.Println("Lock file does not exist yet... Waiting before a new attempt...")
				time.Sleep(100 * time.Millisecond)
				continue
			}
			return nil, fmt.Errorf("failed to acquire lock file %q: %w", filename, err)
		}
		data, err := func() ([]byte, error) {
			defer lockedFile.Close()
			data, err := io.ReadAll(lockedFile)
			if err != nil {
				return nil, err
			}
			return data, nil
		}()
		if err != nil {
			return nil, fmt.Errorf("failed reading contents of lock file %q: %w", filename, err)
		}
		if len(data) > 0 {
			log.Printf("Read lock acquired, and contains %q! Proceeding...\n", string(data))
			return data, nil
		}
		log.Println("Read lock acquired, but is empty... waiting for other process to proceed...")
		time.Sleep(100 * time.Millisecond)
	}
}
