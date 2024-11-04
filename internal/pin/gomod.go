// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package pin

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
)

type (
	goMod struct {
		Go        string
		Toolchain string
		Require   []goModRequire
		Replace   []goModReplace
	}

	goModRequire struct {
		Path    string
		Version string
	}

	goModReplace struct {
		Old goModModule
		New goModModule
	}

	goModModule struct {
		Path    string
		Version string
	}
)

func parse(filename string) (goMod, error) {
	cmd := exec.Command("go", "mod", "edit", "-json")
	cmd.Env = append(os.Environ(), "GOMOD="+filename)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return goMod{}, fmt.Errorf("creating json output pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return goMod{}, fmt.Errorf("spawning `go mod edit -json`: %w", err)
	}
	defer cmd.Wait()

	var mod goMod
	err = json.NewDecoder(stdout).Decode(&mod)
	if err != nil {
		return goMod{}, fmt.Errorf("decoding output of `go mode edit -json`: %w", err)
	}

	if mod.Toolchain == "" {
		// If there is no `mod.Toolchain`, we'll set it to `"none"` to indicate
		// explicit absence, as this is what you need to specify to
		// `go mod edit -toolchain=` in order to get rid of that directive.
		mod.Toolchain = "none"
	}

	return mod, nil
}

func (m *goMod) requires(path string) bool {
	for _, r := range m.Require {
		if r.Path == path {
			return true
		}
	}
	return false
}

func (m *goMod) replaces(path string) bool {
	for _, r := range m.Replace {
		if r.Old.Path == path {
			return true
		}
	}
	return false
}
