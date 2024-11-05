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
	"github.com/DataDog/orchestrion/internal/pin"
	"gopkg.in/yaml.v3"
)

type yamlSource struct {
	extends  []Config
	aspects  []aspect.Aspect
	filename string
}

// loadYAMLFromDir loads the [pin.OrchestrionDotYML] file from the provided directory, if one
// exists. If the file does not exist, returns nil and no error.
func (l *loader) loadYAMLFromDir(dir string) (*yamlSource, error) {
	src, err := l.loadYAML(filepath.Join(dir, pin.OrchestrionDotYML))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	return src, err
}

func (l *loader) loadYAML(filename string) (*yamlSource, error) {
	if l.markLoaded(filename) {
		return nil, nil
	}

	decoded, err := parseYAML(filename)
	if err != nil {
		return nil, err
	}

	extends := make([]Config, 0, len(decoded.Extends))
	for _, ext := range decoded.Extends {
		extFilename := ext
		if !filepath.IsAbs(extFilename) {
			extFilename = filepath.Join(filename, "..", ext)
		}

		stat, err := os.Stat(extFilename)
		if err != nil {
			return nil, fmt.Errorf("stat %q: %w", ext, maskNotExists(err))
		}

		if stat.IsDir() {
			cfg, err := l.loadPackage(".", extFilename)
			if err != nil {
				return nil, fmt.Errorf("loading package in %q: %w", ext, err)
			}
			if cfg == nil || cfg.Empty() {
				continue
			}
			extends = append(extends, cfg)
		} else {
			cfg, err := l.loadYAML(extFilename)
			if err != nil {
				return nil, fmt.Errorf("extends %q: %w", ext, maskNotExists(err))
			}
			if cfg == nil || cfg.Empty() {
				continue
			}
			cfg.filename = ext
			extends = append(extends, cfg)
		}
	}

	src := &yamlSource{
		extends:  extends,
		aspects:  decoded.Aspects,
		filename: filepath.Base(filename),
	}

	return src, nil
}

func (s *yamlSource) Aspects() []aspect.Aspect {
	var aspects []aspect.Aspect

	for _, ext := range s.extends {
		aspects = append(aspects, ext.Aspects()...)
	}
	aspects = append(aspects, s.aspects...)

	return aspects
}

func (s *yamlSource) Empty() bool {
	return len(s.aspects) == 0 && len(s.extends) == 0
}

type yamlSchema struct {
	Extends []string        `yaml:"extends"`
	Aspects []aspect.Aspect `yaml:"aspects"`
}

func parseYAML(filename string) (yamlSchema, error) {
	var decoded yamlSchema

	file, err := os.Open(filename)
	if err != nil {
		return decoded, fmt.Errorf("opening %q: %w", filename, err)
	}

	decoder := yaml.NewDecoder(file)
	if err := decoder.Decode(&decoded); err != nil {
		return decoded, fmt.Errorf("decoding %q: %w", filename, err)
	}

	return decoded, nil
}

func maskNotExists(err error) error {
	if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return fmt.Errorf("%v", err)
}
