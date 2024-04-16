// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package aspect

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"

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
	for _, archive := range reg.PackageFile {
		data, err := readArchiveData(archive, linkdeps.LinkDepsFilename)
		if err != nil {
			return fmt.Errorf("reading %s from %q: %w", linkdeps.LinkDepsFilename, archive, err)
		} else if data == nil {
			continue
		}

		log.Tracef("Found %s file in %q\n", linkdeps.LinkDepsFilename, archive)
		linkDeps, err := linkdeps.Read(data)
		if err != nil {
			return fmt.Errorf("reading %s from %q: %w", linkdeps.LinkDepsFilename, archive, err)
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

// readArchiveData returns the content of the given entry from the provided archive file. If there
// is no such entry in the archive, a nil io.Reader and no error is returned.
func readArchiveData(archive, entry string) (io.Reader, error) {
	var list, data bytes.Buffer
	cmd := exec.Command("go", "tool", "pack", "t", archive)
	cmd.Stdout = &list
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("running `go tool pack t %q`: %w", archive, err)
	}
	for {
		line, err := list.ReadString('\n')
		if err == io.EOF {
			return nil, nil
		}
		if err != nil {
			return nil, fmt.Errorf("reading pack list from %q: %w", archive, err)
		}
		if line[:len(line)-1] == entry {
			// Found it!
			break
		}
	}

	cmd = exec.Command("go", "tool", "pack", "p", archive, entry)
	cmd.Stdout = &data
	return &data, cmd.Run()
}
