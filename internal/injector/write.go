// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package injector

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/DataDog/orchestrion/internal/injector/lineinfo"
	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
	"github.com/rs/zerolog"
)

// writeModifiedFile writes the modified file to disk after having restored it to Go source code,
// and returns the path to the modified file.
func (i *Injector) writeModifiedFile(ctx context.Context, decorator *decorator.Decorator, file *dst.File) (string, error) {
	log := zerolog.Ctx(ctx)

	filename := decorator.Filenames[file]

	log.Trace().Str("path", filename).Msg("Writing modified file")
	canonicalizeImports(ctx, file)

	writtenAs := filename
	if i.ModifiedFile != nil {
		writtenAs = i.ModifiedFile(filename)
		if dir := filepath.Dir(writtenAs); dir != "." {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return writtenAs, fmt.Errorf("mkdir %q: %w", dir, err)
			}
		}
	}

	// The line directives are computed from the layout that is actually printed, and verified
	// under the path the file is written to, so that the printer cannot change the layout after
	// the positions were measured.
	out, err := lineinfo.FormatFile(lineinfo.Options{
		Decorator:   decorator,
		File:        file,
		NewRestorer: i.newRestorer,
		WrittenAs:   writtenAs,
		PostProcess: postProcess,
		Log:         log,
	})
	if err != nil {
		return writtenAs, fmt.Errorf("annotating moved nodes in %q: %w", writtenAs, err)
	}

	if err := os.WriteFile(writtenAs, out, 0o644); err != nil {
		return writtenAs, fmt.Errorf("writing %q: %w", writtenAs, err)
	}

	return writtenAs, nil
}
