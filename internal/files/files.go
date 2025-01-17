// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package files

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/rs/zerolog"
)

// Copy creates a copy of `oldname` at `newname`.
func Copy(ctx context.Context, oldname string, newname string) error {
	log := zerolog.Ctx(ctx).With().
		Str("oldname", oldname).
		Str("newname", newname).
		Logger()

	in, err := os.Open(oldname)
	if err != nil {
		return fmt.Errorf("open %q: %w", oldname, err)
	}
	defer in.Close()

	out, err := os.Create(newname)
	if err != nil {
		return fmt.Errorf("create %q: %w", newname, err)
	}
	defer out.Close()

	bytes, err := io.Copy(out, in)
	if err != nil {
		return fmt.Errorf("copy %q to %q: %w", oldname, newname, err)
	}

	log.Trace().
		Int64("size", bytes).
		Msg("Successfully copied file over")

	return nil
}
