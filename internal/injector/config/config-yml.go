// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/DataDog/orchestrion/internal/injector/aspect"
	"gopkg.in/yaml.v3"
)

const OrchestrionYML = "orchestrion.yml"

// loadYMLFile loads configuration from the specified directory.
func (l *Loader) loadYMLFile(dir string, name string) (*configYML, error) {
	filename := name
	if !filepath.IsAbs(name) {
		filename = filepath.Join(dir, name)
	}
	if !l.markLoaded(filename) {
		// Already loaded, ignoring...
		return nil, nil
	}

	yml, err := l.parseYMLFile(filename)
	if err != nil {
		return nil, err
	}

	dir = filepath.Dir(filename)
	extends := make([]Config, 0, len(yml.Extends))
	for _, ext := range yml.Extends {
		extFilename := filepath.Join(dir, ext)

		if stat, err := os.Stat(extFilename); err != nil {
			return nil, maskErrNotExist(err)
		} else if stat.IsDir() {
			pkgs, err := l.packages(extFilename)
			if err != nil {
				return nil, fmt.Errorf("extends %q: %w", ext, err)
			}
			if len(pkgs) != 1 {
				// This is not supposed to happen if `err == nil`.
				panic(fmt.Errorf("extends %q: no package returned by packages.Load(%q)", ext, l.dir))
			}

			cfg, err := l.loadGoPackage(pkgs[0])
			if err != nil {
				return nil, maskErrNotExist(err)
			}
			if cfg == nil {
				// Empty, nothing to do...
				continue
			}
			extends = append(extends, cfg)
			continue
		}

		cfg, err := l.loadYMLFile(dir, ext)
		if err != nil {
			return nil, maskErrNotExist(err)
		}
		if cfg == nil {
			// Empty, nothing to do...
			continue
		}
		extends = append(extends, cfg)
	}

	return &configYML{name: name, extends: extends, aspects: yml.Aspects}, nil
}

type configYML struct {
	extends []Config
	aspects []*aspect.Aspect
	name    string
}

func (c *configYML) Aspects() []*aspect.Aspect {
	var res []*aspect.Aspect

	for _, ext := range c.extends {
		res = append(res, ext.Aspects()...)
	}

	res = append(res, c.aspects...)

	return res
}

type ymlFile struct {
	Aspects []*aspect.Aspect
	Extends []string
}

func (l *Loader) parseYMLFile(filename string) (*ymlFile, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("open %q: %w", filename, err)
	}
	defer file.Close()

	// In validation mode, we will pre-parse the YAML into a [yaml.Node] tree,
	// which can then be cheaply decoded into a value type that validation
	// supports; and then re-decoded into the actual data structure.
	// This dance is significantly cheaper (both in time & allocations) than doing
	// a full blown [yaml.Decoder.Decode] twice (as it internally transits through
	// the [yaml.Node] representation anyway).
	var dec interface{ Decode(any) error } = yaml.NewDecoder(file)
	if l.validate {
		var node yaml.Node
		if err := dec.Decode(&node); err != nil {
			return nil, fmt.Errorf("yaml.Decode %q -> yaml.Node: %w", filename, err)
		}
		dec = &node

		var simple map[string]any
		if err := dec.Decode(&simple); err != nil {
			return nil, fmt.Errorf("yaml.Decode %q -> map[string]any: %w", filename, err)
		}

		if err := ValidateObject(simple); err != nil {
			return nil, fmt.Errorf("validate %q: %w", filename, err)
		}
	}

	var yml ymlFile
	if err := dec.Decode(&yml); err != nil {
		return nil, fmt.Errorf("yaml.Decode %q: %w", filename, err)
	}

	return &yml, nil
}

// maskErrNotExist intentionally "breaks" the error chaining if the provided
// error is an [fs.ErrNotExist] so that the returned error is not
// [fs.ErrNotExist]. Otherwise, returns the original error unmodified.
func maskErrNotExist(err error) error {
	if errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("%v", err)
	}
	return err
}
