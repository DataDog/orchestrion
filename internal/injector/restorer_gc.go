// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

//go:build gc

package injector

import (
	"go/types"
	"io"

	"golang.org/x/tools/go/gcexportdata"
)

// readPackageInfo extracts package information from the provided reader.
func (r *lookupResolver) readPackageInfo(rd io.Reader, path string) (*types.Package, error) {
	rd, err := gcexportdata.NewReader(rd)
	if err != nil {
		return nil, err
	}
	pkg, err := gcexportdata.Read(rd, r.fset, r.imports, path)
	if err != nil {
		return nil, err
	}
	return pkg, nil
}
