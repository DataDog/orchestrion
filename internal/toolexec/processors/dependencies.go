// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package processors

import (
	"fmt"
	"go/token"
	"go/types"
	"os"

	"golang.org/x/tools/go/gcexportdata"
)

func listDependencies(archive string, importPath string) (imports map[string]*types.Package, err error) {
	file, err := os.Open(archive)
	if err != nil {
		return nil, fmt.Errorf("opening archive %q: %w", archive, err)
	}
	defer file.Close()

	reader, err := gcexportdata.NewReader(file)
	if err != nil {
		return nil, fmt.Errorf("creating reader for archive %q: %w", archive, err)
	}

	fset := token.NewFileSet()
	imports = make(map[string]*types.Package)
	_, err = gcexportdata.Read(reader, fset, imports, importPath)
	if err != nil {
		return nil, fmt.Errorf("reading gc export data for archive %q: %w", archive, err)
	}

	return
}
