// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package aspect

import (
	"context"
	"fmt"
	"os"

	"github.com/DataDog/orchestrion/internal/toolexec/aspect/linkdeps"
	"github.com/DataDog/orchestrion/internal/toolexec/importcfg"
	"github.com/DataDog/orchestrion/internal/toolexec/proxy"
	"github.com/rs/zerolog"
)

func (Weaver) OnLink(ctx context.Context, cmd *proxy.LinkCommand) error {
	log := zerolog.Ctx(ctx).With().Str("phase", "link").Logger()
	ctx = log.WithContext(ctx)

	reg, err := importcfg.ParseFile(cmd.Flags.ImportCfg)
	if err != nil {
		return fmt.Errorf("parsing %q: %w", cmd.Flags.ImportCfg, err)
	}

	var changed bool
	for archiveImportPath, archive := range reg.PackageFile {
		linkDeps, err := linkdeps.FromArchive(archive)
		if err != nil {
			return fmt.Errorf("reading %s from %q: %w", linkdeps.Filename, archiveImportPath, err)
		}

		log.Debug().Str("import-path", archiveImportPath).Str("archive", archive).Msg("Processing " + linkdeps.Filename + " dependencies")
		for _, depPath := range linkDeps.Dependencies() {
			if arch, found := reg.PackageFile[depPath]; found {
				log.Debug().Str("import-path", depPath).Str("archive", arch).Msg("Already satisfied " + linkdeps.Filename + " dependency")
				continue
			}

			log.Tracef("Resolving %s dependency on %q...\n", linkdeps.Filename, depPath)
			deps, err := resolvePackageFiles(ctx, depPath, cmd.WorkDir)
			if err != nil {
				return fmt.Errorf("resolving %q: %w", depPath, err)
			}
			for p, a := range deps {
				if _, found := reg.PackageFile[p]; !found {
					log.Debug().Str("import-path", p).Str("archive", a).Msg("Recording resolved " + linkdeps.Filename + " dependency")
					reg.PackageFile[p] = a
					changed = true
				}
			}
		}
	}

	if !changed {
		return nil
	}

	log.Trace().Str("path", cmd.Flags.ImportCfg).Msg("Backing up original file")
	if err := os.Rename(cmd.Flags.ImportCfg, cmd.Flags.ImportCfg+".original"); err != nil {
		return fmt.Errorf("renaming %q: %w", cmd.Flags.ImportCfg, err)
	}
	log.Trace().Str("path", cmd.Flags.ImportCfg).Msg("Writing updated file")
	if err := reg.WriteFile(cmd.Flags.ImportCfg); err != nil {
		return fmt.Errorf("writing updated %q: %w", cmd.Flags.ImportCfg, err)
	}

	return nil
}
