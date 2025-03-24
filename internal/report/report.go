// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package report

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/sergi/go-diff/diffmatchpatch"
	"github.com/urfave/cli/v2"
	"golang.org/x/sync/errgroup"
)

// EnvVar is the environment variable that orchestrion uses to tell the job server if it has to do a report
const EnvVar = "ORCHESTRION_REPORT"

// ModifiedFile is a struct that stores the synthetic diff that orchestrion created between two files
type ModifiedFile struct {
	// OriginalFilePath on disk of the original file inside the temp work directory
	OriginalFilePath string `json:"original_file_path"`
	// ModifiedFilePath on disk of the modified file inside the temp work directory
	ModifiedFilePath string `json:"modified_file_path"`
	// ImportPath is the import path of where the file that was modified
	ImportPath string `json:"import_path"`
}

// Report is
type Report struct {
	Files []ModifiedFile `json:"diff"`

	mu   sync.Mutex
	path string
}

// NewEmptyReport makes sure the path does not already exist and returns a new report to be filled
func NewEmptyReport(path string) (*Report, error) {
	_, err := os.Stat(path)
	if err == nil {
		return nil, cli.Exit(fmt.Errorf("file %s already exists, please remove it before running orchestrion with the report flag", path), 1)
	}

	if !os.IsNotExist(err) {
		return nil, cli.Exit(fmt.Errorf("stat %s: %w", path, err), 1)
	}

	if !filepath.IsAbs(path) {
		wd, err := os.Getwd()
		if err != nil {
			return nil, cli.Exit(fmt.Errorf("getwd: %w", err), 1)
		}
		path = filepath.Join(wd, path)
	}

	return &Report{
		path: path,
	}, nil
}

// ParseReport reads a report from a file
func ParseReport(path string) (*Report, error) {
	fp, err := os.Open(path)
	if err != nil {
		return nil, cli.Exit(fmt.Errorf("open %s: %w", path, err), 1)
	}

	defer fp.Close()

	var report Report
	if err := json.NewDecoder(fp).Decode(&report); err != nil {
		return nil, cli.Exit(fmt.Errorf("decode %s: %w", path, err), 1)
	}

	return &report, nil
}

// Append adds a new file diff to the report
func (r *Report) Append(files ...ModifiedFile) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Files = append(r.Files, files...)
}

func (r *Report) Save() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	fp, err := os.Create(r.path)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %s\n", r.path, err)
	}

	defer fp.Close()

	encoder := json.NewEncoder(fp)
	encoder.SetIndent("", "  ")

	if err := encoder.Encode(r); err != nil {
		return fmt.Errorf("failed to encode report: %s\n", err)
	}

	return nil
}

// Diff reads the files and create the actual diff between them and print them to the output.
func (r *Report) Diff(writer io.Writer) error {
	dmp := diffmatchpatch.New()

	var (
		wg      errgroup.Group
		diffs   []diffmatchpatch.Diff
		diffsMu sync.Mutex
	)

	for _, file := range r.Files {
		wg.Go(func() error {
			source, err := os.ReadFile(file.OriginalFilePath)
			if err != nil {
				return fmt.Errorf("read %s: %w", file.OriginalFilePath, err)
			}

			target, err := os.ReadFile(file.ModifiedFilePath)
			if err != nil {
				return fmt.Errorf("read %s: %w", file.ModifiedFilePath, err)
			}

			fragments := dmp.DiffMain(string(source), string(target), false)
			fragments = dmp.DiffCleanupSemantic(fragments)
			diffsMu.Lock()
			defer diffsMu.Unlock()
			diffs = append(diffs, fragments...)
			return nil
		})
	}

	if err := wg.Wait(); err != nil {
		return err
	}

	output := dmp.DiffPrettyText(diffs)
	length := len(output)

	for {
		if length == 0 {
			break
		}
		n, err := io.WriteString(writer, output)
		if err != nil {
			return err
		}
		length -= n
	}

	return nil
}
