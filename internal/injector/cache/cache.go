// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

// Package cache provides a persistent content-addressed cache for instrumented
// Go source files. It allows orchestrion to skip the full AST transformation
// pipeline for packages whose inputs (source files, aspects configuration,
// import map, etc.) have not changed since the last instrumentation.
package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/DataDog/orchestrion/internal/version"
)

// CacheDir returns the directory used for the persistent instrumentation cache.
func CacheDir() string {
	dir := os.Getenv("ORCHESTRION_CACHE_DIR")
	if dir != "" {
		return dir
	}
	home, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, "orchestrion", "inject", "v1")
}

// Key uniquely identifies a package's instrumentation inputs.
type Key struct {
	hash string
}

// ComputeKey computes a cache key from all inputs that affect instrumentation.
func ComputeKey(
	importPath string,
	goVersion string,
	testMain bool,
	goFiles []string,
	aspectsConfigFiles []string,
	importPaths []string,
) Key {
	h := sha256.New()
	fmt.Fprintf(h, "orchestrion:%s\n", version.Tag())
	fmt.Fprintf(h, "import-path:%s\n", importPath)
	fmt.Fprintf(h, "go-version:%s\n", goVersion)
	fmt.Fprintf(h, "test-main:%t\n", testMain)

	// Hash source file contents (order-independent via sorting).
	sortedGoFiles := make([]string, len(goFiles))
	copy(sortedGoFiles, goFiles)
	sort.Strings(sortedGoFiles)
	for _, f := range sortedGoFiles {
		fh := hashFile(f)
		fmt.Fprintf(h, "source:%s:%s\n", f, fh)
	}

	// Hash aspects config file contents.
	sortedAspects := make([]string, len(aspectsConfigFiles))
	copy(sortedAspects, aspectsConfigFiles)
	sort.Strings(sortedAspects)
	for _, f := range sortedAspects {
		fh := hashFile(f)
		fmt.Fprintf(h, "aspect:%s:%s\n", f, fh)
	}

	// Hash the set of available import paths (sorted for determinism).
	sortedImports := make([]string, len(importPaths))
	copy(sortedImports, importPaths)
	sort.Strings(sortedImports)
	fmt.Fprintf(h, "imports:%s\n", strings.Join(sortedImports, ","))

	return Key{hash: hex.EncodeToString(h.Sum(nil))}
}

// Entry represents a cached instrumentation result for a package.
type Entry struct {
	// ModifiedFiles maps original file path to its cached modification.
	ModifiedFiles map[string]ModifiedFile `json:"modifiedFiles,omitempty"`
	// GoLang is the minimum Go language version required by injected code.
	GoLang string `json:"goLang,omitempty"`
}

// ModifiedFile contains the cached content and references for a single
// instrumented source file.
type ModifiedFile struct {
	// Content is the instrumented Go source code.
	Content []byte `json:"content"`
	// References maps import paths to whether they are ImportStatement (true)
	// or RelocationTarget (false).
	References map[string]bool `json:"references,omitempty"`
}

// Lookup checks the cache for a hit. Returns nil if not found.
func Lookup(key Key) *Entry {
	dir := CacheDir()
	if dir == "" {
		return nil
	}

	path := filepath.Join(dir, key.hash[:2], key.hash+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	var entry Entry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil
	}
	return &entry
}

// Store writes a cache entry to disk.
func Store(key Key, entry *Entry) {
	dir := CacheDir()
	if dir == "" {
		return
	}

	path := filepath.Join(dir, key.hash[:2], key.hash+".json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return
	}

	// Atomic write: write to temp file, then rename.
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return
	}
	os.Rename(tmp, path)
}

func hashFile(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return "err:" + err.Error()
	}
	defer f.Close()

	h := sha256.New()
	io.Copy(h, f)
	return hex.EncodeToString(h.Sum(nil))
}
