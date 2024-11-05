// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package config

// Load reads all configuration found in the provided directory. This will
// attempt to read [pin.OrchestrionToolGo] and [pin.OrchestrionDotYML] files,
// and register any found entity as a source of configuration.
func Load(dir string, validate bool) (Config, error) {
	loader := loader{validate: validate}
	return loader.loadPackage(".", dir)
}

type loader struct {
	dedup    map[string]struct{}
	validate bool
}

// markLoaded returns true if the filename was already loaded; and marks it as
// loaded and returns false otherwise.
func (l *loader) markLoaded(filename string) bool {
	if _, found := l.dedup[filename]; found {
		return true
	}
	if l.dedup == nil {
		l.dedup = make(map[string]struct{})
	}
	l.dedup[filename] = struct{}{}
	return false
}
