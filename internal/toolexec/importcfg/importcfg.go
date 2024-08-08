// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

// Package importcfg provides utilities to deal with files accepted by the Go toolchain commands as
// the `-importcfg` flag value.
package importcfg

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

// ImportConfig represents the parsed out contents of an `importcfg` (or `importcfg.link`) file,
// usually passed to the Go compiler and linker via the `-importcfg` flag.
type ImportConfig struct {
	// PackageFile maps package dependencies fully-qualified import paths to their build archive
	// location
	PackageFile map[string]string
	// ImportMap maps package dependencies import paths to their fully-qualified version
	ImportMap map[string]string
	// Extras is data read from an `importcfg` file that is not semantically parsed by this data
	// structure, which is stored only so it can be written back with an updated `importcfg` file if
	// necessary.
	Extras []string
}

// ParseFile parses the contents of the provided `importcfg` (or `importcfg.link`) file.
func ParseFile(filename string) (ImportConfig, error) {
	file, err := os.Open(filename)
	if err != nil {
		return ImportConfig{}, err
	}
	defer file.Close()

	return parse(file)
}

// ParseFile parses the `importcfg` (or `importcfg.link`) data from the provided reader.
func parse(r io.Reader) (reg ImportConfig, err error) {
	scanner := bufio.NewScanner(r)
	scanner.Split(bufio.ScanLines)

	for scanner.Scan() {
		if err = scanner.Err(); err != nil {
			return
		}

		line := strings.TrimSpace(scanner.Text())
		if line == "" || line[0] == '#' {
			continue
		}

		directive, data, ok := strings.Cut(line, " ")
		if !ok {
			reg.Extras = append(reg.Extras, line)
			continue
		}

		switch directive {
		case "packagefile":
			importPath, archive, ok := strings.Cut(data, "=")
			if !ok {
				reg.Extras = append(reg.Extras, line)
				continue
			}

			if reg.PackageFile == nil {
				reg.PackageFile = make(map[string]string)
			}
			reg.PackageFile[importPath] = archive

		case "importmap":
			importPath, mappedTo, ok := strings.Cut(data, "=")
			if !ok {
				reg.Extras = append(reg.Extras, line)
				continue
			}

			if reg.ImportMap == nil {
				reg.ImportMap = make(map[string]string)
			}
			reg.ImportMap[importPath] = mappedTo

		default:
			reg.Extras = append(reg.Extras, line)
		}
	}

	return
}

// Lookup opens the archive file for the provided import path.
func (r *ImportConfig) Lookup(path string) (io.ReadCloser, error) {
	lookup := path
	if p, mapped := r.ImportMap[lookup]; mapped {
		lookup = p
	}
	filename := r.PackageFile[lookup]
	if filename == "" {
		suffix := ""
		if lookup != path {
			suffix = fmt.Sprintf(" (as %q)", lookup)
		}
		return nil, fmt.Errorf("no package file found for %q%s", path, suffix)
	}
	return os.Open(filename)
}

// CombinePackageFile copies `packagefile` entries from other into the receiver unless it already
// has an entry with the same import path.
func (r *ImportConfig) CombinePackageFile(other *ImportConfig) (changed bool) {
	for k, v := range other.PackageFile {
		if _, ok := r.PackageFile[k]; !ok {
			r.PackageFile[k] = v
			changed = true
		}
	}
	return
}

// WriteFile writes the content of the package register to the provided file, in the format expected
// by the standard go toolchain commands.
func (r *ImportConfig) WriteFile(filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	return r.write(file)
}

// write writes the content of the package register to the provided writer, in the format expected
// by the standard go toolchain commands.
func (r *ImportConfig) write(w io.Writer) error {
	for name, path := range r.ImportMap {
		_, err := fmt.Fprintf(w, "importmap %s=%s\n", name, path)
		if err != nil {
			return err
		}
	}

	for name, path := range r.PackageFile {
		_, err := fmt.Fprintf(w, "packagefile %s=%s\n", name, path)
		if err != nil {
			return err
		}
	}

	for _, data := range r.Extras {
		_, err := fmt.Fprintf(w, "%s\n", data)
		if err != nil {
			return err
		}
	}

	return nil
}
