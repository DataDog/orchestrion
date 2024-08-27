// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package importcfg

import (
	"fmt"
	"io"
	"os"
)

// Lookup opens the archive file for the provided import path. This allows the
// ImportConfig object to serve as a package information resolver's Lookup
// function.
func (r *ImportConfig) Lookup(path string) (io.ReadCloser, error) {
	lookup := path
	if p, mapped := r.ImportMap[lookup]; mapped {
		lookup = p
	}

	filename := r.PackageFile[lookup]
	if filename == "" {
		err := fmt.Errorf("no package file found for %q", lookup)
		if lookup != path {
			err = fmt.Errorf("mapped from %q: %w", path, err)
		}
		return nil, err
	}

	return os.Open(filename)
}
