// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

// Package injector provides a facility to inject code into go programs, either
// in source (intended to be checked in by the user) or at compilation time
// (via `-toolexec`).
package injector

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/format"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"

	"github.com/datadog/orchestrion/internal/injector/aspect"
	"github.com/datadog/orchestrion/internal/injector/lineinfo"
	"github.com/datadog/orchestrion/internal/injector/node"
	"github.com/datadog/orchestrion/internal/injector/typed"
	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
	"github.com/dave/dst/decorator/resolver"
	"github.com/dave/dst/decorator/resolver/gotypes"
	"github.com/dave/dst/dstutil"
)

type (
	// Injector injects go code into a program.
	Injector struct {
		// Aspects is the set of configured injections to attempt.
		Aspects []aspect.Aspect
		// ModifiedFile is called to obtain the file name to use when writing a modified file.
		// If nil, the original file is overwritten in-place.
		ModifiedFile ModifiedFileFn
		// LookupImport is a function used to resolve package import paths.
		LookupImport importer.Lookup

		// RootConfig is the root configuration value.
		RootConfig map[string]string

		// ImportPath is the import path for the package being injected.
		ImportPath string
		// Name is the name of the package being injected.
		// If blank, the package name is determined from parsing source files.
		Name string
		// GoVersion is the go version level to use for parsing & type checking the source code.
		// If blank, no go version check will be performed.
		GoVersion string

		// PreserveLineInfo enables emission of //line directives to preserve line information from the original file, so
		// that stack traces resolve to the original source code. This is strongly recommended when performing compile-time
		// injection.
		PreserveLineInfo bool

		// restorerResolver is the resolver to use when restoring modified files. It is created on-demand then stored in
		// this field so it can be re-used.
		restorerResolver resolver.RestorerResolver
	}

	// ModifiedFileFn is called with the original file and must return the path to use when writing a modified version.
	ModifiedFileFn func(string) string

	// Result describes the result of an injection operation.
	Result struct {
		References typed.ReferenceMap // New package references injected into the file and what kind of reference they are
		Filename   string             // The file name of the output file (may be different from the input file)
		Modified   bool               // Whether the file was modified
	}
)

func (i *Injector) validate() error {
	if i.ImportPath == "" {
		return errors.New("invalid *Injector: ImportPath is required")
	}
	if i.LookupImport == nil {
		return errors.New("invalid *Injector: LookupImport is required")
	}
	return nil
}

// InjectFiles performs code injection on all specified files. It returns one result for each input file; or an error.
func (i *Injector) InjectFiles(goFiles []string) ([]Result, error) {
	if err := i.validate(); err != nil {
		return nil, err
	}

	fset := token.NewFileSet()
	astFiles, err := i.parseFiles(fset, goFiles)
	if err != nil {
		return nil, err
	}

	uses, err := i.typeCheckFiles(fset, astFiles)
	if err != nil {
		return nil, err
	}

	dec := decorator.NewDecoratorWithImports(fset, i.ImportPath, gotypes.New(uses))
	dstFiles := make([]*dst.File, len(astFiles))
	for idx, astFile := range astFiles {
		var err error
		dstFiles[idx], err = dec.DecorateFile(astFile)
		if err != nil {
			return nil, err
		}
	}

	results := make([]Result, len(dstFiles))
	for idx, dstFile := range dstFiles {
		results[idx], err = i.injectFile(dstFile, dec)
		if err != nil {
			return nil, err
		}
	}

	return results, nil
}

func (i *Injector) parseFiles(fset *token.FileSet, filenames []string) ([]*ast.File, error) {
	astFiles := make([]*ast.File, len(filenames))
	for idx, filename := range filenames {
		var err error
		astFiles[idx], err = i.parseFile(fset, filename)
		if err != nil {
			return nil, err
		}
	}
	return astFiles, nil
}

// Matches the line (and optionally column) information off a line directive.
var reLineCol = regexp.MustCompile(`[:]\d+([:]\d+)?$`)

func (*Injector) parseFile(fset *token.FileSet, filename string) (*ast.File, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// If the file starts with a `//line` directive, we consume it and then proceed with the file as
	// if its name was the one from the directive.
	if fileLineCol, err := consumeLineDirective(file); err != nil {
		return nil, err
	} else if fileLineCol != "" {
		filename = reLineCol.ReplaceAllLiteralString(fileLineCol, "")
	}

	astFile, err := parser.ParseFile(fset, filename, file, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	return astFile, nil
}

// consumeLineDirective consumes the first line from `r` if it is a `//line` directive, and returns
// the adjusted file name with line and column information. If the first line is not a `//line`
// directive, it rewinds the reader to absolute offset 0, and returns a blank string.
func consumeLineDirective(r io.ReadSeeker) (string, error) {
	var buf [7]byte
	n, err := r.Read(buf[:])
	if err != nil {
		return "", err
	}

	// Is this a line directive?
	if string(buf[:n]) != "//line " {
		// Rewind to the start of the file
		if _, err := r.Seek(0, io.SeekStart); err != nil {
			return "", fmt.Errorf("seeking to start of file: %w", err)
		}
		return "", nil
	}

	var filename []byte
	var carriageReturn bool
	for {
		if n, err := r.Read(buf[:1]); err != nil {
			return "", err
		} else if n == 0 {
			return string(filename), nil
		}
		c := buf[0]
		switch c {
		case '\n':
			return string(filename), nil
		case '\r':
			carriageReturn = true
		default:
			// Previous char was CR, so we roll back one & break out...
			if carriageReturn {
				if _, err := r.Seek(-1, io.SeekCurrent); err != nil {
					return "", err
				}
				return string(filename), nil
			}
			filename = append(filename, c)
		}
	}
}

func (i *Injector) typeCheckFiles(fset *token.FileSet, files []*ast.File) (map[*ast.Ident]types.Object, error) {
	importer := importer.ForCompiler(
		fset,
		runtime.Compiler,
		i.LookupImport,
	)
	pkg := types.NewPackage(i.ImportPath, i.Name)
	typeInfo := types.Info{Uses: make(map[*ast.Ident]types.Object)}

	checker := types.NewChecker(
		&types.Config{GoVersion: i.GoVersion, Importer: importer},
		fset,
		pkg,
		&typeInfo,
	)
	return typeInfo.Uses, checker.Files(files)
}

func (i *Injector) injectFile(file *dst.File, dec *decorator.Decorator) (Result, error) {
	result := Result{Filename: dec.Fset.Position(dec.Ast.Nodes[file].Pos()).Filename}

	ctx := typed.ContextWithValue(context.Background(), dec)
	var err error
	if result.Modified, result.References, err = i.inject(ctx, file, dec, i.RootConfig); err != nil {
		return result, err
	}

	if result.Modified {
		result.Filename, err = i.writeModifiedFile(dec, file)
		if err != nil {
			return result, err
		}
	}

	return result, nil
}

func (i *Injector) newRestorer(filename string) *decorator.FileRestorer {
	if i.restorerResolver == nil {
		i.restorerResolver = &lookupResolver{lookup: i.LookupImport}
	}

	return &decorator.FileRestorer{
		Restorer: decorator.NewRestorerWithImports(
			i.ImportPath,
			i.restorerResolver,
		),
		Name: filename,
	}
}

func (i *Injector) writeModifiedFile(dec *decorator.Decorator, file *dst.File) (string, error) {
	filename := dec.Filenames[file]

	// Ensure the restorer does not break due to multiple imports of the same package
	normalizeImports(file)

	if i.PreserveLineInfo {
		var err error
		file, err = lineinfo.AnnotateMovedNodes(dec, file, i.newRestorer)
		if err != nil {
			return filename, err
		}
	}

	res := i.newRestorer(filename)
	astFile, err := res.RestoreFile(file)
	if err != nil {
		return filename, fmt.Errorf("restoring %q: %w", filename, err)
	}

	var buf bytes.Buffer
	if err := format.Node(&buf, res.Fset, astFile); err != nil {
		return "", fmt.Errorf("formatting AST of %q: %w", filename, err)
	}

	if i.ModifiedFile != nil {
		filename = i.ModifiedFile(filename)
		filedir := filepath.Dir(filename)
		if err := os.MkdirAll(filedir, 0o755); err != nil {
			return filename, fmt.Errorf("creating %q: %w", filedir, err)
		}
	}
	if err := os.WriteFile(filename, postProcess(buf.Bytes()), 0o644); err != nil {
		return "", fmt.Errorf("writing %q: %w", filename, err)
	}

	return filename, nil
}

// inject performs all configured injections on the specified file. It returns whether the file was
// modified, any import references introduced by modifications. In case of an error, the
// trasnformation aborts as quickly as possible and returns the error.
func (i *Injector) inject(ctx context.Context, file *dst.File, decorator *decorator.Decorator, rootConfig map[string]string) (mod bool, refs typed.ReferenceMap, err error) {
	ctx = typed.ContextWithValue(ctx, &refs)
	var chain *node.Chain

	dstutil.Apply(
		file,
		func(csor *dstutil.Cursor) bool {
			if err != nil || csor.Node() == nil || ddIgnored(csor.Node()) {
				return false
			}
			chain = chain.ChildFromCursor(csor, decorator.Path)
			if _, ok := csor.Node().(*dst.File); ok {
				// This is the root node, so we set the root configuration on it...
				for k, v := range rootConfig {
					chain.SetConfig(k, v)
				}
			}
			return true
		},
		func(csor *dstutil.Cursor) bool {
			if err != nil || csor.Node() == nil || ddIgnored(csor.Node()) {
				return false
			}

			// Pop the ancestry stack now that we're done with this node.
			defer func() { chain = chain.Parent() }()

			var changed bool
			changed, err = i.injectNode(ctx, chain, csor)
			mod = mod || changed

			return err == nil
		},
	)

	// We only inject synthetic imports here because it may offset declarations by one position in
	// case a new import declaration is necessary, which causes dstutil.Apply to re-traverse the
	// current declaration.
	if refs.AddSyntheticImports(file) {
		mod = true
	}

	return
}

// injectNode assesses all configured injections agaisnt the current node, and performs any AST
// transformations. It returns whether the AST was indeed modified. In case of an error, the
// injector aborts immediately and returns the error.
func (i *Injector) injectNode(ctx context.Context, chain *node.Chain, csor *dstutil.Cursor) (mod bool, err error) {
	for _, inj := range i.Aspects {
		if !inj.JoinPoint.Matches(chain) {
			continue
		}
		for _, act := range inj.Advice {
			var changed bool
			changed, err = act.Apply(ctx, chain, csor)
			if changed {
				mod = true
			}
			if err != nil {
				return
			}
		}
	}

	return
}
