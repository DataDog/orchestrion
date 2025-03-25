// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package linkdeps

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/DataDog/dd-trace-go/v2/ddtrace/tracer"
	"github.com/DataDog/orchestrion/internal/toolexec/importcfg"
	"github.com/blakesmith/ar"
	"github.com/rs/zerolog"
)

const (
	// Filename is the standard file name for link.deps files.
	Filename = "link.deps"

	headerV1 = "#" + Filename + "@v1"
)

// LinkDeps represents the contents of a [Filename] file. It lists all synthetic
// dependencies added by instrumentation into a Go object archive. These include
// the transitive closure of link-time dependencies, so that it is not necessary
// to perform a full traversal of transitive dependencies to consume. Link-time
// dependencies consist of new dependencies introduced to resolve go:linkname
// directives as well as new import-level directives that the Go toolchain is
// not normally aware of.
type LinkDeps struct {
	deps map[string]struct{}
}

// FromImportConfig aggregates entries from all [Filename] found in the
// archives listed in [importcfg.ImportConfig].
func FromImportConfig(ctx context.Context, importcfg *importcfg.ImportConfig) (LinkDeps, error) {
	var res LinkDeps

	span, ctx := tracer.StartSpanFromContext(ctx, "LinkDeps.FromImportConfig")
	defer span.Finish()

	for importPath, archivePath := range importcfg.PackageFile {
		ld, err := FromArchive(ctx, archivePath)
		if err != nil {
			return LinkDeps{}, fmt.Errorf("reading %s from %s=%s: %w", Filename, importPath, archivePath, err)
		}

		for dep := range ld.deps {
			if _, satisfied := importcfg.PackageFile[dep]; satisfied {
				// This transitive link-time dependency is already satisfied at
				// compile-time, so we don't need to carry it over.
				continue
			}
			res.Add(dep)
		}
	}

	return res, nil
}

// FromArchive reads a [Filename] file from the provided Go archive file.
// Returns an empty [LinkDeps] if the archive does not contain a [Filename]
// file.
func FromArchive(ctx context.Context, archive string) (res LinkDeps, err error) {
	span, _ := tracer.StartSpanFromContext(ctx, "linkdeps.FromArchive",
		tracer.ResourceName(archive),
	)
	defer func() { span.Finish(tracer.WithError(err)) }()

	var data io.ReadCloser
	data, err = readArchiveData(archive, Filename)
	if err != nil {
		return res, fmt.Errorf("reading %s from %q: %w", Filename, archive, err)
	}
	if data == nil {
		return
	}
	defer data.Close()
	return Read(data)
}

// ReadFile reads a [Filename] file from the provided filename.
func ReadFile(filename string) (LinkDeps, error) {
	file, err := os.Open(filename)
	if err != nil {
		return LinkDeps{}, err
	}
	defer file.Close()

	return Read(file)
}

// Read reads a [Filename] file content from the provided [io.Reader].
func Read(r io.Reader) (l LinkDeps, err error) {
	rd := bufio.NewReader(r)

	var line string
	if line, err = rd.ReadString('\n'); err != nil {
		return
	}

	switch hdr := strings.TrimSpace(line); hdr {
	case headerV1:
		return parseV1(rd)
	default:
		err = fmt.Errorf("unsupported data format %q, a newer Orchestion release may be required", hdr)
		return
	}
}

// parseV1 parses the contents of V1 [Filename] files.
func parseV1(r *bufio.Reader) (l LinkDeps, err error) {
	for {
		var line string
		if line, err = r.ReadString('\n'); err != nil {
			if err == io.EOF {
				err = nil
				return
			}
			return
		}

		if strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		l.Add(line)
	}
}

// Contains checks whether a given import path is already represented by this
// [LinkDeps].
func (l *LinkDeps) Contains(importPath string) bool {
	_, found := l.deps[importPath]
	return found
}

// Add registers a new import path in this [LinkDeps] instance.
func (l *LinkDeps) Add(importPath string) {
	if l.deps == nil {
		l.deps = make(map[string]struct{})
	}
	l.deps[importPath] = struct{}{}
}

// Dependencies returns all import paths registered in this [LinkDeps] instance.
func (l *LinkDeps) Dependencies() []string {
	deps := make([]string, 0, len(l.deps))
	for importPath := range l.deps {
		deps = append(deps, importPath)
	}
	return deps
}

// Empty returns true if this [LinkDeps] instance is empty.
func (l *LinkDeps) Empty() bool {
	return l.Len() == 0
}

// Len returns the number of import paths registered in this [LinkDeps]
// instance.
func (l *LinkDeps) Len() int {
	return len(l.deps)
}

// Write writes this [LinkDeps] instance to the provided writer.
func (l *LinkDeps) Write(w io.Writer) error {
	if _, err := fmt.Fprintln(w, headerV1); err != nil {
		return err
	}

	// We sort entries to ensure the output is deterministic, since these files
	// eventually get embedded in `_pkg_.a` files and we wouldn't want to cause
	// unnecessary rebuilds.
	sorted := make([]string, 0, len(l.deps))
	for dep := range l.deps {
		sorted = append(sorted, dep)
	}
	sort.Strings(sorted)

	for _, dep := range sorted {
		if _, err := fmt.Fprintln(w, dep); err != nil {
			return err
		}
	}

	return nil
}

// readArchiveData returns the content of the given entry from the provided archive file. If there
// is no such entry in the archive, a nil io.Reader and no error is returned.
func readArchiveData(archive string, entry string) (rc io.ReadCloser, err error) {
	file, err := os.Open(archive)
	if err != nil {
		return nil, fmt.Errorf("opening archive: %w", err)
	}
	defer func() {
		// If we return no [io.ReadCloser], then we need to close the file ourselves.
		if rc != nil {
			return
		}
		err = errors.Join(err, file.Close())
	}()

	rd := ar.NewReader(file)
	for {
		hdr, err := rd.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("reading archive: %w", err)
		}
		if hdr.Name != entry {
			continue
		}

		return arReadCloser{Reader: rd, Closer: file}, nil
	}

	return nil, nil
}

var _ zerolog.LogArrayMarshaler = (*LinkDeps)(nil)

func (l *LinkDeps) MarshalZerologArray(a *zerolog.Array) {
	for _, dep := range l.Dependencies() {
		a.Str(dep)
	}
}

type arReadCloser struct {
	*ar.Reader
	io.Closer
}
