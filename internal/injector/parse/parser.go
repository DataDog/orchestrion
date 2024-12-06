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

	// rawFileEagerness is a channel that receives files that are not parsed because we are not sure we need to parse them
	// because no aspects has match the package yet.
	// this means that all files in have no aspects to run on, they are only there of the type checking
	rawFileEagerness      []rawFile
	rawFileEagernessIndex atomic.Int32

	// filesBytesCount is the total number of bytes of all the files that have been read so far.
	// if this number is more than maxBytesEagerness, we stop using rawFileEagerness and parse all the files.
	filesBytesCount atomic.Uint64

	// Output fields
	parsedFiles      []File
	parsedFilesIndex atomic.Int32

	wg sync.WaitGroup

	errs      []error
	errsIndex atomic.Int32
}

func NewParser(fset *token.FileSet, nbFiles int) *Parser {
	p := &Parser{
		fset:             fset,
		rawFileEagerness: make([]rawFile, nbFiles),
		parsedFiles:      make([]File, nbFiles),
		errs:             make([]error, nbFiles),
	}

	return p
}

// ParseFiles return either zero files or all files parsed with their respective aspects
func (p *Parser) ParseFiles(files []string, aspects []*aspect.Aspect) ([]File, error) {
	p.wg.Add(len(files))

	for _, file := range files {
		fileAspects := make([]*aspect.Aspect, len(aspects))
		copy(fileAspects, aspects)

		go func() {
			defer p.wg.Done()
			rawFile, err := readFile(file)
			if err != nil {
				p.addError(fmt.Errorf("reading %q: %w", file, err))
				return
			}

			p.filesBytesCount.Add(uint64(len(rawFile.content)))
			if p.parsedFilesIndex.Load() > 0 && p.filesBytesCount.Load() <= maxBytesEagerness {
				fileAspects = p.fileFilterAspects(fileAspects, rawFile)
				if len(fileAspects) == 0 {
					p.storeRawFile(rawFile)
					return
				}
			}

			p.parseFile(rawFile, fileAspects)
		}()
	}

	p.wg.Wait()

	if p.parsedFilesIndex.Load() == 0 {
		return nil, errors.Join(p.errs...)
	}

	// Parse eager files if at least one goroutine decided to parse it's own file because type-checking requires all
	// files of the package
	p.goParseEagerFiles()
	p.wg.Wait()

	return p.parsedFiles, errors.Join(p.errs...)
}

func (p *Parser) addError(err error) {
	p.errs[p.errsIndex.Add(1)-1] = err
}

func (p *Parser) storeRawFile(rawFile rawFile) {
	p.rawFileEagerness[p.rawFileEagernessIndex.Add(1)-1] = rawFile
}

func (p *Parser) parseFile(rawFile rawFile, aspects []*aspect.Aspect) {
	astFile, err := goparser.ParseFile(p.fset, rawFile.mappedName, rawFile.content, goparser.ParseComments)
	if err != nil {
		p.addError(fmt.Errorf("parsing %q: %w", rawFile.name, err))
		return
	}

	p.parsedFiles[p.parsedFilesIndex.Add(1)-1] = File{rawFile.name, astFile, aspects}
}

func (p *Parser) goParseEagerFiles() {
	for _, file := range p.rawFileEagerness {
		// at this point, the array is not written to anymore but "empty" files from the initialization are in there
		if file.name == "" {
			continue
		}

		p.wg.Add(1)
		go func(file rawFile) {
			defer p.wg.Done()
			p.parseFile(file, nil)
		}(file)
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

	return slices.DeleteFunc(aspects, func(a *aspect.Aspect) (res bool) {
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
