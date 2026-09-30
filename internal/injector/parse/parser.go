// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package parse

import (
	"context"
	"fmt"
	"go/ast"
	goparser "go/parser"
	"go/token"
	"io"
	"os"
	"slices"
	"sync/atomic"

	"github.com/DataDog/dd-trace-go/v2/ddtrace/tracer"
	"github.com/DataDog/orchestrion/internal/injector/aspect"
	"github.com/DataDog/orchestrion/internal/injector/aspect/may"
	"golang.org/x/sync/errgroup"
)

// maxBytesEagerness is the size of a package's files, beyond which the package is parsed (and hence
// type-checked by the injector) even if no aspect may match on any of its files. Since 99% of packages
// have no aspect that may match on them, skipping them saves a lot of time; but historically, packages
// larger than this were always parsed (the per-file filtering was deemed not worth it), and this is
// preserved so that errors in such packages (e.g, syntax errors, or a Go version newer than
// orchestrion supports) keep being reported by orchestrion.
const maxBytesEagerness = 1 << 19 // 512 KiB

type rawFile struct {
	name       string
	mappedName string
	content    []byte
}

// File represents a parsed file with its name, its AST and with the aspects to apply to it.
type File struct {
	// Name is the name of the file.
	Name string
	// AstFile is the parsed AST of the file, cannot be nil
	AstFile *ast.File
	// Aspects is the list of aspects to apply to this file: empty if none may match on it, and all of
	// them otherwise, as advice may add code to the file that any aspect may match on.
	Aspects []*aspect.Aspect
}

type Parser struct {
	fset *token.FileSet // thread-safe data structure

	// rawFiles is an intermediary data structure to store the raw content of the files before parsing them.
	rawFiles []rawFile

	// hasCandidates is set once any file has at least one aspect that may match on it, in which case
	// all files must be parsed (as the type-checking pass needs them).
	hasCandidates atomic.Bool

	// filesBytesCount is the sum of the sizes of the files read so far.
	filesBytesCount atomic.Uint64

	// parsedFiles is what is returned by ParseFiles.
	parsedFiles []File

	wg errgroup.Group
}

// NewParser creates a new parser with the given [token.FileSet] and the number of files to parse.
func NewParser(fset *token.FileSet, nbFiles int) *Parser {
	return &Parser{
		fset:        fset,
		rawFiles:    make([]rawFile, nbFiles),
		parsedFiles: make([]File, nbFiles),
	}
}

// ParseFiles return either zero files if no aspect may match on any file of the package (unless the
// package is larger than [maxBytesEagerness]), or all files parsed with the aspects to apply to them
// (see [File.Aspects]).
func (p *Parser) ParseFiles(ctx context.Context, files []string, aspects []*aspect.Aspect) ([]File, error) {
	for idx, file := range files {
		idx, file := idx, file
		p.wg.Go(func() error {
			var err error
			p.rawFiles[idx], err = readFile(file)
			if err != nil {
				return fmt.Errorf("reading %q: %w", file, err)
			}
			p.filesBytesCount.Add(uint64(len(p.rawFiles[idx].content)))

			// Find out whether any aspect may match on this file before parsing it. Most files have none,
			// and are then left untouched. The others get all aspects rather than only those: advice may
			// add code to the file (e.g, `inject-declarations`), which is not part of the content that was
			// just checked, and which any aspect may match on.
			mayMatch, err := anyAspectMayMatch(aspects, p.rawFiles[idx])
			if err != nil {
				return fmt.Errorf("filtering aspects for %q: %w", file, err)
			}
			if !mayMatch {
				// No aspects can match on this file, no need to fill up the File.AstFile field (yet).
				p.parsedFiles[idx] = File{Name: file}
				return nil
			}

			p.hasCandidates.Store(true)
			p.parsedFiles[idx], err = p.parseFile(ctx, p.rawFiles[idx], aspects)
			return err
		})
	}

	if err := p.wg.Wait(); err != nil {
		return nil, err
	}

	// No aspects can match on this package, return nothing
	if !p.mustParseAll() {
		return nil, nil
	}

	// If we arrived here, this means we need to parse all files anyway because the type-checking pass will need them.
	if err := p.parseMissingFiles(ctx); err != nil {
		return nil, err
	}

	return p.parsedFiles, nil
}

// mustParseAll returns true if all files of the package must be parsed, either because an aspect may
// match on at least one of them, or because the package is larger than [maxBytesEagerness].
func (p *Parser) mustParseAll() bool {
	return p.hasCandidates.Load() || p.filesBytesCount.Load() > maxBytesEagerness
}

func (p *Parser) parseFile(ctx context.Context, rawFile rawFile, aspects []*aspect.Aspect) (File, error) {
	span, _ := tracer.StartSpanFromContext(ctx, "Parser.parseFile",
		tracer.ResourceName(rawFile.mappedName),
	)
	defer span.Finish()

	astFile, err := goparser.ParseFile(p.fset, rawFile.mappedName, rawFile.content, goparser.ParseComments)
	if err != nil {
		return File{}, fmt.Errorf("parsing %q: %w", rawFile.name, err)
	}

	return File{rawFile.name, astFile, aspects}, nil
}

func (p *Parser) parseMissingFiles(ctx context.Context) error {
	for i := range p.parsedFiles {
		// Skip files that have already been parsed.
		if p.parsedFiles[i].AstFile != nil {
			continue
		}

		i := i
		p.wg.Go(func() error {
			var err error
			p.parsedFiles[i], err = p.parseFile(ctx, p.rawFiles[i], nil)
			return err
		})
	}

	return p.wg.Wait()
}

// anyAspectMayMatch reports whether any of the aspects may match on the file, according to
// [join.Point.FileMayMatch]. It returns an error if the file's package clause cannot be parsed, even if
// there are no aspects.
func anyAspectMayMatch(aspects []*aspect.Aspect, file rawFile) (bool, error) {
	// The package clause's AST is discarded right away, so it need not be recorded in a shared FileSet.
	astFile, err := goparser.ParseFile(token.NewFileSet(), file.mappedName, file.content, goparser.PackageClauseOnly)
	if err != nil {
		return false, fmt.Errorf("parsing package clause %q: %w", file.name, err)
	}

	if astFile.Name == nil {
		return false, fmt.Errorf("no package name found in %q", file.name)
	}

	ctx := &may.FileContext{
		FileContent: file.content,
		PackageName: astFile.Name.Name,
	}

	return slices.ContainsFunc(aspects, func(a *aspect.Aspect) bool {
		return a.JoinPoint.FileMayMatch(ctx) != may.NeverMatch
	}), nil
}

func readFile(filename string) (rawFile, error) {
	file, err := os.Open(filename)
	if err != nil {
		return rawFile{}, fmt.Errorf("open %q: %w", filename, err)
	}
	defer file.Close()

	// If the file begins with a "//line <path>:1:1" directive, we consume it and
	// then pretend the "<path>" was our filename all along. This simplifies
	// handling of line offsets further down the line and removes some duplicated
	// effort to do it early.
	mappedFilename := filename
	if mapped, err := ConsumeLineDirective(file); err != nil {
		return rawFile{}, fmt.Errorf("peeking at first line of %q: %w", filename, err)
	} else if mapped != "" {
		mappedFilename = mapped
	}

	fileContent, err := io.ReadAll(file)
	if err != nil {
		return rawFile{}, fmt.Errorf("reading %q: %w", filename, err)
	}

	return rawFile{filename, mappedFilename, fileContent}, nil
}

// AnyFileMayMatch reports whether [Parser.ParseFiles] would return any file: that is, whether at least
// one of the aspects may match on at least one of the files (according to [join.Point.FileMayMatch]),
// or whether the files are larger than [maxBytesEagerness] in total. It returns true whenever it cannot
// tell (e.g, if a file cannot be read, or if its package clause cannot be parsed), so that callers fall
// back to [Parser.ParseFiles], which reports such errors.
func AnyFileMayMatch(files []string, aspects []*aspect.Aspect) bool {
	var size uint64
	for _, file := range files {
		raw, err := readFile(file)
		if err != nil {
			return true
		}
		if size += uint64(len(raw.content)); size > maxBytesEagerness {
			return true
		}
		if mayMatch, err := anyAspectMayMatch(aspects, raw); err != nil || mayMatch {
			return true
		}
	}
	return false
}
