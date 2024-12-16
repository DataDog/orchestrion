// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package parse

import (
	"errors"
	"fmt"
	"go/ast"
	goparser "go/parser"
	"go/token"
	"io"
	"os"
	"slices"
	"sync"
	"sync/atomic"

	"github.com/DataDog/orchestrion/internal/injector/aspect"
	"github.com/DataDog/orchestrion/internal/injector/aspect/may"
)

const maxBytesEagerness = 1 << 19 // 512 KiB

type rawFile struct {
	name       string
	mappedName string
	content    []byte
}

type File struct {
	Name    string
	AstFile *ast.File
	Aspects []*aspect.Aspect
}

type Parser struct {
	fset *token.FileSet // thread-safe data structure

	// rawFiles is an intermediary data structure to store the raw content of the files before parsing them.
	rawFiles []rawFile

	// filesBytesCount is the sum of the bytes of all files parsed so far.
	filesBytesCount atomic.Uint64

	// eagerness is a flag that is set to true if at least one file has been parsed and requires all files to be parsed.
	eagerness atomic.Bool

	// parsedFiles is what is returned by ParseFiles.
	parsedFiles []File

	wg sync.WaitGroup

	errs   []error
	errsMu sync.Mutex
}

func NewParser(fset *token.FileSet, nbFiles int) *Parser {
	p := &Parser{
		fset:        fset,
		rawFiles:    make([]rawFile, nbFiles),
		parsedFiles: make([]File, nbFiles),
	}

	p.eagerness.Store(true)

	return p
}

// ParseFiles return either zero files or all files parsed with their respective aspects
func (p *Parser) ParseFiles(files []string, aspects []*aspect.Aspect) ([]File, error) {
	p.wg.Add(len(files))

	for idx, file := range files {
		fileAspects := make([]*aspect.Aspect, len(aspects))
		copy(fileAspects, aspects)

		go func(idx int, file string) {
			defer p.wg.Done()
			var err error
			p.rawFiles[idx], err = readFile(file)
			if err != nil {
				p.addError(fmt.Errorf("reading %q: %w", file, err))
				return
			}

			p.filesBytesCount.Add(uint64(len(p.rawFiles[idx].content)))
			if p.aspectMayMatch() {
				fileAspects = p.fileFilterAspects(fileAspects, p.rawFiles[idx])
				if len(fileAspects) == 0 {
					// No aspects can match on this file, no need to fill up the File.AstFile field.
					p.parsedFiles[idx] = File{Name: file}
					return
				}
			}

			p.eagerness.Store(false)
			p.parsedFiles[idx] = p.parseFile(p.rawFiles[idx], fileAspects)
		}(idx, file)
	}

	p.wg.Wait()

	// No aspects can match on this package, return nothing
	if !p.aspectMayMatch() {
		return nil, nil
	}

	if len(p.errs) > 0 {
		return nil, errors.Join(p.errs...)
	}

	// If we arrived here, this means we need to parse all files anyway because the type-checking pass will need them.
	p.parseMissingFiles()
	p.wg.Wait()

	return p.parsedFiles, errors.Join(p.errs...)
}

// aspectMayMatch returns true if the parser should parse all files because at least one file requires it.
func (p *Parser) aspectMayMatch() bool {
	return !p.eagerness.Load() || p.filesBytesCount.Load() > maxBytesEagerness
}

func (p *Parser) addError(err error) {
	p.errsMu.Lock()
	defer p.errsMu.Unlock()
	p.errs = append(p.errs, err)
}

func (p *Parser) parseFile(rawFile rawFile, aspects []*aspect.Aspect) File {
	astFile, err := goparser.ParseFile(p.fset, rawFile.mappedName, rawFile.content, goparser.ParseComments)
	if err != nil {
		p.addError(fmt.Errorf("parsing %q: %w", rawFile.name, err))
		return File{}
	}

	return File{rawFile.name, astFile, aspects}
}

func (p *Parser) parseMissingFiles() {
	for i := range p.parsedFiles {
		// Skip files that have already been parsed.
		if p.parsedFiles[i].AstFile != nil {
			continue
		}

		p.wg.Add(1)
		go func(i int) {
			defer p.wg.Done()
			p.parsedFiles[i] = p.parseFile(p.rawFiles[i], nil)
		}(i)
	}
}

// fileFilterAspects filters out aspects for a specific file.
func (p *Parser) fileFilterAspects(aspects []*aspect.Aspect, file rawFile) []*aspect.Aspect {
	astFile, err := goparser.ParseFile(p.fset, file.mappedName, file.content, goparser.PackageClauseOnly)
	if err != nil {
		p.addError(fmt.Errorf("parsing package clause %q: %w", file.name, err))
		return nil
	}

	if astFile.Name == nil {
		p.addError(fmt.Errorf("no package name found in %q", file.name))
		return nil
	}

	ctx := &may.FileContext{
		FileContent: file.content,
		PackageName: astFile.Name.Name,
	}

	return slices.DeleteFunc(aspects, func(a *aspect.Aspect) bool {
		return a.JoinPoint.FileMayMatch(ctx) == may.CantMatch
	})
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
	if mapped, err := consumeLineDirective(file); err != nil {
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
