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

// maxBytesEagerness is the maximum number of bytes the files of a certain package can have before
// stop we decide to stop trying to run join.Point.FileMayMatch on each file.
// Since 99% of package have no aspects that ACTUALLY match on them, we can save a lot of time by
// applying the join.Point.FileMayMatch heuristic. But if the package has a lot of files, we may
// end up parsing all files anyway so we can just skip this heuristic if the package is too big.
const maxBytesEagerness = 1 << 19 // 512 KiB

type rawFile struct {
	name       string
	mappedName string
	content    []byte
}

// File represents a parsed file with its name, its AST and with the aspects that may match on it.
type File struct {
	// Name is the name of the file.
	Name string
	// AstFile is the parsed AST of the file, cannot be nil
	AstFile *ast.File
	// Aspects is the list of aspects that may match on this file.
	Aspects []*aspect.Aspect
}

type Parser struct {
	fset *token.FileSet // thread-safe data structure

	// rawFiles is an intermediary data structure to store the raw content of the files before parsing them.
	rawFiles []rawFile

	// filesBytesCount is the sum of the bytes of all files parsed so far.
	filesBytesCount atomic.Uint64

	// mustParseAll is a flag that is set to true if at least one file has been parsed.
	// at this point all files must be parsed. It also signals that an aspect matched on a file.
	mustParseAll atomic.Bool

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

// ParseFiles return either zero files if no aspect matched on any file of the package,
// or all files parsed with their respective aspects that can match on them.
func (p *Parser) ParseFiles(ctx context.Context, files []string, aspects []*aspect.Aspect) ([]File, error) {
	for idx, file := range files {
		idx, file := idx, file
		p.wg.Go(func() error {
			var err error
			p.rawFiles[idx], err = readFile(file)
			if err != nil {
				return fmt.Errorf("reading %q: %w", file, err)
			}

			fileAspects := aspects
			p.filesBytesCount.Add(uint64(len(p.rawFiles[idx].content)))
			if !p.hasApplicableAspects() {
				// While the current package still has a chance to not require all files to be parsed, we can try to filter out
				// aspects that cannot match on this file before parsing it.
				fileAspects, err = p.fileFilterAspects(fileAspects, p.rawFiles[idx])
				if err != nil {
					return fmt.Errorf("filtering aspects for %q: %w", file, err)
				}
				if len(fileAspects) == 0 {
					// No aspects can match on this file, no need to fill up the File.AstFile field.
					p.parsedFiles[idx] = File{Name: file}
					return nil
				}
			}

			p.mustParseAll.Store(true)
			p.parsedFiles[idx], err = p.parseFile(ctx, p.rawFiles[idx], fileAspects)
			return err
		})
	}

	if err := p.wg.Wait(); err != nil {
		return nil, err
	}

	// No aspects can match on this package, return nothing
	if !p.hasApplicableAspects() {
		return nil, nil
	}

	// If we arrived here, this means we need to parse all files anyway because the type-checking pass will need them.
	if err := p.parseMissingFiles(ctx); err != nil {
		return nil, err
	}

	return p.parsedFiles, nil
}

// hasApplicableAspects returns true if the parser should parse all files because at least one file requires it.
func (p *Parser) hasApplicableAspects() bool {
	return p.mustParseAll.Load() || p.filesBytesCount.Load() > maxBytesEagerness
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

// fileFilterAspects filters out aspects for a specific file and returns a copy of them
func (p *Parser) fileFilterAspects(aspects []*aspect.Aspect, file rawFile) ([]*aspect.Aspect, error) {
	astFile, err := goparser.ParseFile(p.fset, file.mappedName, file.content, goparser.PackageClauseOnly)
	if err != nil {
		return nil, fmt.Errorf("parsing package clause %q: %w", file.name, err)
	}

	if astFile.Name == nil {
		return nil, fmt.Errorf("no package name found in %q", file.name)
	}

	ctx := &may.FileContext{
		FileContent: file.content,
		PackageName: astFile.Name.Name,
	}

	copyAspects := make([]*aspect.Aspect, len(aspects))
	copy(copyAspects, aspects)

	return slices.DeleteFunc(copyAspects, func(a *aspect.Aspect) bool {
		return a.JoinPoint.FileMayMatch(ctx) == may.NeverMatch
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
