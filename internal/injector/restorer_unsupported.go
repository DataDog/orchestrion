// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

//go:build !gc

package injector

import (
	"errors"
	"fmt"
	"go/types"
	"io"
	"runtime"
)

// readPackageInfo extracts package information from the provided reader.
func (r *lookupResolver) readPackageInfo(rd io.Reader, path string) (*types.Package, error) {
	return nil, fmt.Errorf("%s: %w", runtime.Compiler, errors.ErrUnsupported)
}
