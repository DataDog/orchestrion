// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package linkdeps

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

const (
	// LinkDepsFilename is the standard file name for link.deps files.
	LinkDepsFilename = "link.deps"

	headerV1 = "#" + LinkDepsFilename + "@v1"
)

// LinkDeps represents the contents of a link.deps file.
type LinkDeps struct {
	deps map[string]struct{}
}

// ReadFile reads a link.deps file from the provided filename.
func ReadFile(filename string) (LinkDeps, error) {
	file, err := os.Open(filename)
	if err != nil {
		return LinkDeps{}, err
	}
	defer file.Close()

	return Read(file)
}

// Read reads a link.deps file content from the provided reader.
func Read(r io.Reader) (l LinkDeps, err error) {
	rd := bufio.NewReader(r)

	var line string
	if line, err = rd.ReadString('\n'); err != nil {
		return
	}

	switch hdr := strings.TrimSpace(line); hdr {
	case headerV1:
		return parseV1(rd)
	default:
		err = fmt.Errorf("unsupported data format %q, a newer Orchestion release may be required", hdr)
		return
	}
}

// parseV1 parses the contents of V1 link.deps files.
func parseV1(r *bufio.Reader) (l LinkDeps, err error) {
	for {
		var line string
		if line, err = r.ReadString('\n'); err != nil {
			if err == io.EOF {
				err = nil
				return
			}
			return
		}

		if strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if l.deps == nil {
			l.deps = make(map[string]struct{}, 1)
		}
		l.deps[line] = struct{}{}
	}
}

// Add registers a new import path in this LinkDeps instance.
func (l *LinkDeps) Add(importPath string) {
	if l.deps == nil {
		l.deps = make(map[string]struct{})
	}
	l.deps[importPath] = struct{}{}
}

// Direct returns all import paths registered in this LinkDeps instance that are
// marked as direct.
func (l *LinkDeps) Direct() []string {
	deps := make([]string, 0, len(l.deps))
	for importPath := range l.deps {
		deps = append(deps, importPath)
	}
	return deps
}

// Empty returns true if this LinkDeps instance is empty.
func (l *LinkDeps) Empty() bool {
	return l.Len() == 0
}

// Len returns the number of import paths registered in this LinkDeps instance.
func (l *LinkDeps) Len() int {
	return len(l.deps)
}

// WriteFile writes this LinkDeps instance to the provided filename.
func (l *LinkDeps) WriteFile(filename string) error {
	wr, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer wr.Close()

	return l.Write(wr)
}

// Write writes this LinkDeps instance to the provided writer.
func (l *LinkDeps) Write(w io.Writer) error {
	if _, err := fmt.Fprintln(w, headerV1); err != nil {
		return err
	}

	// We sort entries to ensure the output is deterministic, since these files
	// eventually get embedded in `_pkg_.a` files and we wouldn't want to cause
	// unnecessary rebuilds.
	sorted := make([]string, 0, len(l.deps))
	for dep := range l.deps {
		sorted = append(sorted, dep)
	}
	sort.Strings(sorted)

	for _, dep := range sorted {
		if _, err := fmt.Fprintln(w, dep); err != nil {
			return err
		}
	}

	return nil
}
