// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package proxy

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/blakesmith/ar"
)

const (
	testMainFilename = "testmain.info"
	testMainHeader   = "#orchestrion.testmain@v1"
)

// MarkTestMain records that this compile command produces a generated test-main archive.
func (cmd *CompileCommand) MarkTestMain(packageUnderTest string) {
	cmd.testVariantFor = packageUnderTest
}

func (cmd *CompileCommand) attachTestMain() error {
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
	defer file.Close()

	contents := testMainHeader + "\n" + cmd.testVariantFor + "\n"
	wr := ar.NewWriter(file)
	if err := wr.WriteHeader(&ar.Header{Name: testMainFilename, Mode: 0o644, Size: int64(len(contents))}); err != nil {
		return fmt.Errorf("writing test-main header: %w", err)
	}
	if _, err := io.WriteString(wr, contents); err != nil {
		return fmt.Errorf("writing test-main metadata: %w", err)
	}
	return nil
}

// TestVariantFor reports the package-under-test recorded in the linker's input archive.
func (cmd *LinkCommand) TestVariantFor(_ context.Context) (string, bool, error) {
	var found string
	for _, input := range cmd.Inputs {
		value, ok, err := readTestMain(input)
		if err != nil {
			return "", false, err
		}
		if !ok {
			continue
		}
		if found != "" && found != value {
			return "", false, fmt.Errorf("conflicting test-main targets %q and %q", found, value)
		}
		found = value
	}
	return found, found != "", nil
}

func readTestMain(filename string) (string, bool, error) {
	file, err := os.Open(filename)
	if err != nil {
		return "", false, fmt.Errorf("opening link input %q: %w", filename, err)
	}
	defer file.Close()

	magic := make([]byte, len("!<arch>\n"))
	if _, err := io.ReadFull(file, magic); errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return "", false, nil
	} else if err != nil {
		return "", false, fmt.Errorf("reading link input %q: %w", filename, err)
	}
	if !bytes.Equal(magic, []byte("!<arch>\n")) {
		return "", false, nil
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", false, fmt.Errorf("rewinding link input %q: %w", filename, err)
	}

	rd := ar.NewReader(file)
	for {
		hdr, err := rd.Next()
		if errors.Is(err, io.EOF) {
			return "", false, nil
		}
		if err != nil {
			return "", false, fmt.Errorf("reading link input %q: %w", filename, err)
		}
		if hdr.Name != testMainFilename {
			continue
		}
		scanner := bufio.NewScanner(rd)
		if !scanner.Scan() || scanner.Text() != testMainHeader {
			return "", false, fmt.Errorf("invalid test-main metadata in %q", filename)
		}
		if !scanner.Scan() || strings.TrimSpace(scanner.Text()) == "" {
			return "", false, fmt.Errorf("missing test-main target in %q", filename)
		}
		value := strings.TrimSpace(scanner.Text())
		if scanner.Scan() || scanner.Err() != nil {
			return "", false, fmt.Errorf("invalid test-main metadata in %q", filename)
		}
		return value, true, nil
	}
}
