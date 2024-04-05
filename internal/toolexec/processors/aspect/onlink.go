// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package aspect

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/datadog/orchestrion/internal/toolexec/processors"
	"github.com/datadog/orchestrion/internal/toolexec/proxy"
)

func (w Weaver) OnLink(cmd *proxy.LinkCommand) error {
	reg, err := processors.ParseImportConfig(cmd.Flags.ImportCfg)
	if err != nil {
		return fmt.Errorf("parsing %q: %w", cmd.Flags.ImportCfg, err)
	}

	archives := make([]string, 0, len(reg.PackageFile))
	for _, archive := range reg.PackageFile {
		archives = append(archives, archive)
	}

	var changed bool
	for len(archives) > 0 {
		last := len(archives) - 1
		archive := archives[last]
		archives = archives[:last]

		data, err := archiveData(archive, nameLinkDeps)
		if err != nil {
			return fmt.Errorf("reading %s from %q: %w", nameLinkDeps, archive, err)
		} else if data == nil {
			continue
		}

		rd := bufio.NewReader(data)
		for {
			line, err := rd.ReadString('\n')
			if err == io.EOF {
				break
			} else if err != nil {
				return fmt.Errorf("reading %s from %q: %w", nameLinkDeps, archive, err)
			}
			line = line[:len(line)-1]
			if _, found := reg.PackageFile[line]; found {
				// We already have this dependency, nothing more to do...
				continue
			}
			deps, err := resolvePackageFiles(line)
			if err != nil {
				return fmt.Errorf("resolving %q: %w", line, err)
			}
			for p, a := range deps {
				if _, found := reg.PackageFile[p]; !found {
					reg.PackageFile[p] = a
					archives = append(archives, a)
					changed = true
				}
			}
		}
	}

	if !changed {
		return nil
	}

	if err := os.Rename(cmd.Flags.ImportCfg, cmd.Flags.ImportCfg+".original"); err != nil {
		return fmt.Errorf("renaming %q: %w", cmd.Flags.ImportCfg, err)
	}
	file, err := os.Create(cmd.Flags.ImportCfg)
	if err != nil {
		return fmt.Errorf("creating %q: %w", cmd.Flags.ImportCfg, err)
	}
	defer file.Close()

	if _, err := reg.WriteTo(file); err != nil {
		return fmt.Errorf("writing updated %q: %w", cmd.Flags.ImportCfg, err)
	}

	return nil
}

func archiveData(archive, entry string) (io.Reader, error) {
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
