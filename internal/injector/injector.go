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
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sync"

	"github.com/datadog/orchestrion/internal/injector/aspect"
	"github.com/datadog/orchestrion/internal/injector/builtin"
	"github.com/datadog/orchestrion/internal/injector/node"
	"github.com/datadog/orchestrion/internal/injector/typed"
	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
	"github.com/dave/dst/decorator/resolver/guess"
	"github.com/dave/dst/dstutil"
	"golang.org/x/tools/go/packages"
)

type (
	// Injector injects go code into a program.
	Injector struct {
		fileset   *token.FileSet
		decorator *decorator.Decorator
		restorer  *decorator.Restorer
		opts      Options
		mutex     sync.Mutex // Guards access to InjectFile
	}

	// ModifiedFileFn is called with the original file and must return the path to use when writing a modified version.
	ModifiedFileFn func(string) string

	Options struct {
		// Aspects is the set of configured injections to attempt.
		Aspects []aspect.Aspect
		// Dir is the working directory to use for resolving dependencies, etc... If blank, the current working directory is
		// used.
		Dir string
		// ModifiedFile is called to obtain the file name to use when writing a modified file. If nil, the original file is
		// overwritten in-place.
		ModifiedFile ModifiedFileFn
		// PreserveLineInfo enables emission of //line directives to preserve line information from the original file, so
		// that stack traces resolve to the original source code. This is strongly recommended when performing compile-time
		// injection.
		PreserveLineInfo bool
	}
)

// New creates a new injector with the specified options.
func New(pkgDir string, opts Options) (*Injector, error) {
	fileset := token.NewFileSet()
	cfg := &packages.Config{
		Dir:  opts.Dir,
		Fset: fileset,
		Mode: packages.NeedName |
			packages.NeedFiles |
			packages.NeedCompiledGoFiles |
			packages.NeedImports |
			packages.NeedTypes |
			packages.NeedTypesSizes |
			packages.NeedSyntax |
			packages.NeedTypesInfo,
	}
	var (
		pkgPath     string
		dec         *decorator.Decorator
		restorerMap map[string]string
	)
	if pkgs, err := decorator.Load(cfg, pkgDir); err != nil {
		return nil, err
	} else {
		pkg := pkgs[0]
		switch len(pkg.Errors) {
		case 0:
			// Nothing to do, this is a success!
		case 1:
			return nil, pkg.Errors[0]
		default:
			return nil, fmt.Errorf("%w (and %d more)", pkg.Errors[0], len(pkg.Errors)-1)
		}

		dec = pkg.Decorator
		pkgPath = pkg.PkgPath
		restorerMap = make(map[string]string, len(pkg.Imports)+len(builtin.RestorerMap))
		for path, name := range builtin.RestorerMap {
			restorerMap[path] = name
		}
		for _, imp := range pkg.Imports {
			if imp.Name == "" {
				// Happens when there is an error while processing the import, typically inability to resolve the name due to a
				// typo or something. If we allow blank names in the map, the restorer just removes the qualifiers, which is
				// obviously undesirable.
				continue
			}
			restorerMap[imp.PkgPath] = imp.Name
		}
	}

	return &Injector{
		fileset:   fileset,
		decorator: dec,
		restorer:  decorator.NewRestorerWithImports(pkgPath, guess.WithMap(restorerMap)),
		opts:      opts,
	}, nil
}

type (
	// Result describes the result of an injection operation.
	Result struct {
		References typed.ReferenceMap // New package references injected into the file and what kind of reference they are
		Filename   string             // The file name of the output file (may be different from the input file)
		Modified   bool               // Whether the file was modified
	}
)

// Injects code in the specified file. This method can be called concurrently by multiple goroutines,
// as is guarded by a sync.Mutex.
func (i *Injector) InjectFile(filename string, rootConfig map[string]string) (res Result, err error) {
	i.mutex.Lock()
	defer i.mutex.Unlock()

	res.Filename = filename

	var file *dst.File
	{
		stat, err := os.Stat(filename)
		if err != nil {
			return res, err
		}
		for node, name := range i.decorator.Filenames {
			if name == "" {
				// A bunch of synthetic nodes won't have a file name.
				continue
			}
			s, err := os.Stat(name)
			if err != nil {
				continue
			}
			if os.SameFile(stat, s) {
				file = node
				break
			}
		}
	}

	if file == nil {
		src, err := os.ReadFile(filename)
		if err != nil {
			return res, err
		}

		if file, err = i.decorator.ParseFile(filename, src, parser.ParseComments); err != nil {
			return res, err
		}
	}

	ctx := typed.ContextWithValue(context.Background(), i.decorator)
	if res.Modified, res.References, err = i.inject(ctx, file, rootConfig); err != nil {
		return res, err
	}

	if res.Modified {
		buf := bytes.NewBuffer(nil)
		if err = i.restorer.Fprint(buf, file); err != nil {
			return res, err
		}

		res.Filename = i.outputFileFor(filename)
		if err := os.MkdirAll(filepath.Dir(res.Filename), 0o755); err != nil {
			return res, err
		}
		err = os.WriteFile(res.Filename, postProcess(buf.Bytes()), 0o644)
	}

	return res, err
}

// inject performs all configured injections on the specified file. It returns whether the file was
// modified, any import references introduced by modifications. In case of an error, the
// trasnformation aborts as quickly as possible and returns the error.
func (i *Injector) inject(ctx context.Context, file *dst.File, rootConfig map[string]string) (mod bool, refs typed.ReferenceMap, err error) {
	ctx = typed.ContextWithValue(ctx, &refs)
	var chain *node.Chain

	dstutil.Apply(
		file,
		func(csor *dstutil.Cursor) bool {
			if err != nil || csor.Node() == nil || ddIgnored(csor.Node()) {
				return false
			}
			chain = chain.ChildFromCursor(csor, i.decorator.Path)
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

	if mod && i.opts.PreserveLineInfo {
		i.addLineDirectives(file)
	}

	return
}

// injectNode assesses all configured injections agaisnt the current node, and performs any AST
// transformations. It returns whether the AST was indeed modified. In case of an error, the
// injector aborts immediately and returns the error.
func (i *Injector) injectNode(ctx context.Context, chain *node.Chain, csor *dstutil.Cursor) (mod bool, err error) {
	for _, inj := range i.opts.Aspects {
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
func (i *Injector) addLineDirectives(file *dst.File) {
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

		ast := i.decorator.Ast.Nodes[node]
		if ast != nil {
			position := i.fileset.Position(ast.Pos())
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

// outputFileFor returns the file name to be used when writing a modified file. It uses the options
// specified when building this Injector.
func (i *Injector) outputFileFor(filename string) string {
	if i.opts.ModifiedFile == nil {
		return filename
	}
	return i.opts.ModifiedFile(filename)
}
