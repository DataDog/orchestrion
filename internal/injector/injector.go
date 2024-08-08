// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

// Package injector provides a facility to inject code into go programs, either
// in source (intended to be checked in by the user) or at compilation time
// (via `-toolexec`).
package injector

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"regexp"
	"runtime"

	"github.com/datadog/orchestrion/internal/injector/aspect"
	"github.com/datadog/orchestrion/internal/injector/node"
	"github.com/datadog/orchestrion/internal/injector/typed"
	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
	"github.com/dave/dst/decorator/resolver/gotypes"
	"github.com/dave/dst/dstutil"
	"golang.org/x/tools/go/gccgoexportdata"
	"golang.org/x/tools/go/gcexportdata"
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

	res := decorator.NewRestorerWithImports(i.ImportPath, &resolver{fset: fset, lookup: i.LookupImport})

	results := make([]Result, len(dstFiles))
	for idx, dstFile := range dstFiles {
		results[idx], err = i.injectFile(fset, dstFile, dec, res)
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

var reFileLineCol = regexp.MustCompile(`(?i)^(.+\.go)(?:[:]\d+){0,2}$`)

func (*Injector) parseFile(fset *token.FileSet, filename string) (*ast.File, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// If the first line is a `//line` directive, extract the original file name from it, and use it
	// as the parsed file name instead of the original file system path.
	scanner := bufio.NewScanner(file)
	if scanner.Scan() {
		firstLine := scanner.Bytes()
		if bytes.HasPrefix(firstLine, []byte("//line ")) {
			fileLineCol := firstLine[7:]
			if matches := reFileLineCol.FindSubmatch(fileLineCol); matches != nil {
				filename = string(matches[1])
			}
		}
	}

	// Rewind to the start of the file
	if _, err := file.Seek(0, 0); err != nil {
		return nil, fmt.Errorf("seeking to start of file: %w", err)
	}

	astFile, err := parser.ParseFile(fset, filename, file, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	return astFile, nil
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

func (i *Injector) injectFile(fset *token.FileSet, file *dst.File, dec *decorator.Decorator, rest *decorator.Restorer) (Result, error) {
	res := Result{Filename: fset.Position(dec.Ast.Nodes[file].Pos()).Filename}

	ctx := typed.ContextWithValue(context.Background(), dec)
	var err error
	if res.Modified, res.References, err = i.inject(ctx, fset, file, dec, i.RootConfig); err != nil {
		return res, err
	}

	if res.Modified {
		res.Filename, err = i.writeModifiedFile(res.Filename, file, rest)
		if err != nil {
			return res, err
		}
	}

	return res, nil
}

func (i *Injector) writeModifiedFile(filename string, file *dst.File, rest *decorator.Restorer) (string, error) {
	var buf bytes.Buffer
	if err := rest.Fprint(&buf, file); err != nil {
		return "", fmt.Errorf("restoring %q: %w", filename, err)
	}

	if i.ModifiedFile != nil {
		filename = i.ModifiedFile(filename)
		if err := os.MkdirAll(filepath.Dir(filename), 0o755); err != nil {
			return filename, err
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
func (i *Injector) inject(ctx context.Context, fset *token.FileSet, file *dst.File, decorator *decorator.Decorator, rootConfig map[string]string) (mod bool, refs typed.ReferenceMap, err error) {
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

	if mod && i.PreserveLineInfo {
		i.addLineDirectives(fset, file, decorator)
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

// addLineDirectives travers a transformed AST and adds "//line file:line" directives where
// necessary to preserve the original file's line numbering, and to correctly locate synthetic nodes
// within a `<generated>` pseudo-file.
func (i *Injector) addLineDirectives(fset *token.FileSet, file *dst.File, decorator *decorator.Decorator) {
	var (
		// Whether we are in generated code or not
		inGen = false
		// Force emitting a generated code line directive even if we are already in generated code. This
		// is necessary when original AST nodes are inlined within generated code (usually by
		// a wrap-expression advice), so we appropriately resume generated code tagging afterwards.
		forceGen = false
	)

	var stack []bool
	dst.Inspect(file, func(node dst.Node) bool {
		if node == nil {
			if len(stack) == 0 {
				panic("popping empty stack")
			}
			forceGen = !inGen && stack[len(stack)-1]
			inGen, stack = inGen || stack[len(stack)-1], stack[:len(stack)-1]
			return true
		}

		// Push the current node onto the stack
		defer func() {
			stack = append(stack, inGen)
			// The forceGen flag is reset after any node is processed, as at this stage we have resumed
			// normal operations.
			forceGen = false
		}()

		ast := decorator.Ast.Nodes[node]
		if ast != nil {
			position := fset.Position(ast.Pos())
			// Generated nodes from templates may have a corresponding AST node, with a blank filename.
			// Those should be treated as synthetic nodes (they are!).
			if position.Filename != "" {
				if inGen {
					// We need to properly re-position this node (previous node was synthetic)
					deco := node.Decorations()
					if deco.Before == dst.None {
						deco.Before = dst.NewLine
					}
					deco.Start.Append(fmt.Sprintf("//line %s:%d", position.Filename, position.Line))
					inGen = false
				}
				return true
			}
		}

		if !inGen || forceGen {
			deco := node.Decorations()
			if deco.Before == dst.None {
				deco.Before = dst.NewLine
			}
			deco.Start.Prepend("//line <generated>:1")
			inGen = true
		}

		return true
	})

	if len(stack) != 0 {
		panic("finished with non-zero stack!")
	}
}

type resolver struct {
	fset    *token.FileSet
	lookup  importer.Lookup
	imports map[string]*types.Package
}

// ResolvePackage retrieves the package name from the provided import path.
func (r *resolver) ResolvePackage(path string) (string, error) {
	// Special case -- the "unsafe" package does not have an export file
	if path == "unsafe" {
		return "unsafe", nil
	}

	rd, err := r.lookup(path)
	if err != nil {
		return "", err
	}
	defer rd.Close()

	if r.imports == nil {
		r.imports = make(map[string]*types.Package)
	}

	switch runtime.Compiler {
	case "gc":
		rd, err := gcexportdata.NewReader(rd)
		if err != nil {
			return "", err
		}
		pkg, err := gcexportdata.Read(rd, r.fset, r.imports, path)
		if err != nil {
			return "", err
		}
		return pkg.Name(), nil
	case "gccgo":
		rd, err := gccgoexportdata.NewReader(rd)
		if err != nil {
			return "", err
		}
		pkg, err := gccgoexportdata.Read(rd, r.fset, r.imports, path)
		if err != nil {
			return "", err
		}
		return pkg.Name(), nil
	default:
		return "", fmt.Errorf("%s: %w", runtime.Compiler, errors.ErrUnsupported)
	}
}
