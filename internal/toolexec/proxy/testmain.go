// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package proxy

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"slices"
	"strings"

	goarchive "github.com/DataDog/orchestrion/internal/toolexec/archive"
	"github.com/blakesmith/ar"
)

const (
	testMainFilename = "testmain.info"
	testMainHeaderV1 = "#orchestrion.testmain@v1"
	testMainHeaderV2 = "#orchestrion.testmain@v2"
)

// TestMainInfo describes the package under test and any reverse test-variant
// archives selected before compiling the generated test main.
type TestMainInfo struct {
	Target              string            `json:"target"`
	PackageFiles        map[string]string `json:"packageFiles,omitempty"`
	PackageFingerprints map[string]string `json:"packageFingerprints,omitempty"`
	ReverseRoots        []string          `json:"reverseRoots,omitempty"`
}

// MarkTestMain records that this compile command produces a generated test-main archive.
func (cmd *CompileCommand) MarkTestMain(packageUnderTest string) {
	cmd.testVariantFor = packageUnderTest
}

// SetTestMainPackageFiles records package archives that must replace the outer
// linker's ordinary closure for this test binary.
func (cmd *CompileCommand) SetTestMainPackageFiles(packageFiles map[string]string) {
	cmd.testMainPackageFiles = packageFiles
}

// AddTestMainReverseRoot records a root package used to reconstruct reverse
// test variants when their cached archive paths are no longer available.
func (cmd *CompileCommand) AddTestMainReverseRoot(importPath string) {
	if cmd.testMainReverseRoots == nil {
		cmd.testMainReverseRoots = make(map[string]struct{})
	}
	cmd.testMainReverseRoots[importPath] = struct{}{}
}

func (cmd *CompileCommand) attachTestMain() (err error) {
	if cmd.testVariantFor == "" {
		return nil
	}
	if _, err := os.Stat(cmd.Flags.Output); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	file, err := os.OpenFile(cmd.Flags.Output, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("opening test-main archive: %w", err)
	}
	defer func() { err = errors.Join(err, file.Close()) }()

	fingerprints := make(map[string]string, len(cmd.testMainPackageFiles))
	for importPath, archive := range cmd.testMainPackageFiles {
		fingerprint, err := goarchive.CompatibilityFingerprint(archive)
		if err != nil {
			return fmt.Errorf("fingerprinting reverse test variant %q: %w", importPath, err)
		}
		fingerprints[importPath] = fingerprint
	}
	roots := make([]string, 0, len(cmd.testMainReverseRoots))
	for root := range cmd.testMainReverseRoots {
		roots = append(roots, root)
	}
	slices.Sort(roots)
	payload, err := json.Marshal(TestMainInfo{
		Target:              cmd.testVariantFor,
		PackageFiles:        cmd.testMainPackageFiles,
		PackageFingerprints: fingerprints,
		ReverseRoots:        roots,
	})
	if err != nil {
		return fmt.Errorf("encoding test-main metadata: %w", err)
	}
	contents := testMainHeaderV2 + "\n" + string(payload) + "\n"
	wr := ar.NewWriter(file)
	if err := wr.WriteHeader(&ar.Header{Name: testMainFilename, Mode: 0o644, Size: int64(len(contents))}); err != nil {
		return fmt.Errorf("writing test-main header: %w", err)
	}
	if _, err := io.WriteString(wr, contents); err != nil {
		return fmt.Errorf("writing test-main metadata: %w", err)
	}
	return nil
}

// TestMainInfo reports metadata recorded in the linker's test-main input archive.
func (cmd *LinkCommand) TestMainInfo(_ context.Context) (TestMainInfo, bool, error) {
	var found TestMainInfo
	for _, input := range cmd.Inputs {
		value, ok, err := readTestMain(input)
		if err != nil {
			return TestMainInfo{}, false, err
		}
		if !ok {
			continue
		}
		if found.Target != "" {
			if found.Target != value.Target {
				return TestMainInfo{}, false, fmt.Errorf("conflicting test-main targets %q and %q", found.Target, value.Target)
			}
			if !maps.Equal(found.PackageFiles, value.PackageFiles) ||
				!maps.Equal(found.PackageFingerprints, value.PackageFingerprints) ||
				!slices.Equal(found.ReverseRoots, value.ReverseRoots) {
				return TestMainInfo{}, false, fmt.Errorf("conflicting test-main metadata for %q", found.Target)
			}
		}
		found = value
	}
	return found, found.Target != "", nil
}

// TestVariantFor reports the package-under-test recorded in the linker's input archive.
func (cmd *LinkCommand) TestVariantFor(ctx context.Context) (string, bool, error) {
	info, ok, err := cmd.TestMainInfo(ctx)
	return info.Target, ok, err
}

func readTestMain(filename string) (TestMainInfo, bool, error) {
	file, err := os.Open(filename)
	if err != nil {
		return TestMainInfo{}, false, fmt.Errorf("opening link input %q: %w", filename, err)
	}
	defer file.Close()

	magic := make([]byte, len("!<arch>\n"))
	if _, err := io.ReadFull(file, magic); errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return TestMainInfo{}, false, nil
	} else if err != nil {
		return TestMainInfo{}, false, fmt.Errorf("reading link input %q: %w", filename, err)
	}
	if !bytes.Equal(magic, []byte("!<arch>\n")) {
		return TestMainInfo{}, false, nil
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return TestMainInfo{}, false, fmt.Errorf("rewinding link input %q: %w", filename, err)
	}

	rd := ar.NewReader(file)
	for {
		hdr, err := rd.Next()
		if errors.Is(err, io.EOF) {
			return TestMainInfo{}, false, nil
		}
		if err != nil {
			return TestMainInfo{}, false, fmt.Errorf("reading link input %q: %w", filename, err)
		}
		if hdr.Name != testMainFilename {
			continue
		}
		scanner := bufio.NewScanner(rd)
		scanner.Buffer(nil, 16<<20)
		if !scanner.Scan() {
			return TestMainInfo{}, false, fmt.Errorf("invalid test-main metadata in %q", filename)
		}
		header := scanner.Text()
		if !scanner.Scan() || strings.TrimSpace(scanner.Text()) == "" {
			return TestMainInfo{}, false, fmt.Errorf("missing test-main target in %q", filename)
		}
		line := strings.TrimSpace(scanner.Text())
		var value TestMainInfo
		switch header {
		case testMainHeaderV1:
			value.Target = line
		case testMainHeaderV2:
			if err := json.Unmarshal([]byte(line), &value); err != nil || value.Target == "" {
				return TestMainInfo{}, false, fmt.Errorf("invalid test-main metadata in %q", filename)
			}
		default:
			return TestMainInfo{}, false, fmt.Errorf("invalid test-main metadata in %q", filename)
		}
		if scanner.Scan() || scanner.Err() != nil {
			return TestMainInfo{}, false, fmt.Errorf("invalid test-main metadata in %q", filename)
		}
		return value, true, nil
	}
}
