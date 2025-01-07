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

func LinkOrCopy(ctx context.Context, oldname string, newname string) error {
	err := os.Link(oldname, newname)
	if err == nil {
		return nil
	}
	log := zerolog.Ctx(ctx)
	log.Info().
		Str("oldname", oldname).
		Str("newname", newname).
		Err(err).
		Msg("Could not hard link archive; attempting a copy instead...")

		// Hard link can fail (e.g, if the original file & storage dir are not on the
	// same file system). In those cases, we create a full copy of the file.
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

	if _, err := io.Copy(out, in); err != nil {
		return fmt.Errorf("copy %q to %q: %w", oldname, newname, err)
	}

	return nil
}
