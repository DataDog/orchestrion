// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package injector

import (
	"bytes"
	"fmt"
	"go/format"
	"os"
	"path/filepath"

	"github.com/DataDog/orchestrion/internal/injector/lineinfo"
	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
)

// writeModifiedFile writes the modified file to disk after having restored it to Go source code,
// and returns the path to the modified file.
func (i *Injector) writeModifiedFile(decorator *decorator.Decorator, file *dst.File) (string, error) {
	canonicalizeImports(file)

	filename := decorator.Filenames[file]

	if err := lineinfo.AnnotateMovedNodes(decorator, file, i.newRestorer); err != nil {
		return filename, fmt.Errorf("annotating moved nodes in %q: %w", filename, err)
	}

	restorer := i.newRestorer(filename)
	astFile, err := restorer.RestoreFile(file)
	if err != nil {
		return filename, fmt.Errorf("restoring %q: %w", filename, err)
	}

	var buf bytes.Buffer
	if err := format.Node(&buf, restorer.Fset, astFile); err != nil {
		return filename, fmt.Errorf("formatting %q: %w", filename, err)
	}

	if i.ModifiedFile != nil {
		filename = i.ModifiedFile(filename)
		dir := filepath.Dir(filename)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return filename, fmt.Errorf("mkdir %q: %w", dir, err)
		}
	}

	if err := os.WriteFile(filename, postProcess(buf.Bytes()), 0o644); err != nil {
		return filename, fmt.Errorf("writing %q: %w", filename, err)
	}

	return filename, nil
}
