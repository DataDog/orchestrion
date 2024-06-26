// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package aspect

import (
	"fmt"
	"os"

	"github.com/datadog/orchestrion/internal/log"
	"github.com/datadog/orchestrion/internal/toolexec/aspect/linkdeps"
	"github.com/datadog/orchestrion/internal/toolexec/importcfg"
	"github.com/datadog/orchestrion/internal/toolexec/proxy"
)

func (w Weaver) OnLink(cmd *proxy.LinkCommand) error {
	log.SetContext("PHASE", "link")
	defer log.SetContext("PHASE", "")

	reg, err := importcfg.ParseFile(cmd.Flags.ImportCfg)
	if err != nil {
		return fmt.Errorf("parsing %q: %w", cmd.Flags.ImportCfg, err)
	}

	var changed bool
	for archiveImportPath, archive := range reg.PackageFile {
		linkDeps, err := linkdeps.FromArchive(archiveImportPath, archive)
		if err != nil {
			return err
		}

		for _, depPath := range linkDeps.Dependencies() {
			if arch, found := reg.PackageFile[depPath]; found {
				log.Debugf("Already satisfied %s dependency: %q => %q\n", linkdeps.LinkDepsFilename, depPath, arch)
				continue
			}

			deps, err := resolvePackageFiles(depPath)
			if err != nil {
				return fmt.Errorf("resolving %q: %w", depPath, err)
			}
			for p, a := range deps {
				if _, found := reg.PackageFile[p]; !found {
					log.Debugf("Recording resolved %s dependency: %q => %q\n", linkdeps.LinkDepsFilename, p, a)
					reg.PackageFile[p] = a
					changed = true
				}
			}
		}
	}

	if !changed {
		return nil
	}

	log.Tracef("Backing up original %q\n", cmd.Flags.ImportCfg)
	if err := os.Rename(cmd.Flags.ImportCfg, cmd.Flags.ImportCfg+".original"); err != nil {
		return fmt.Errorf("renaming %q: %w", cmd.Flags.ImportCfg, err)
	}
	log.Tracef("Writing updated %q\n", cmd.Flags.ImportCfg)
	if err := reg.WriteFile(cmd.Flags.ImportCfg); err != nil {
		return fmt.Errorf("writing updated %q: %w", cmd.Flags.ImportCfg, err)
	}

	return nil
}
