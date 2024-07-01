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
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sync"

	"github.com/datadog/orchestrion/internal/goflags"
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
		fileset    *token.FileSet
		decorators []*decorator.Decorator
		restorer   *decorator.Restorer
		opts       Options
		mutex      sync.Mutex // Guards access to InjectFile
	}

	// ModifiedFileFn is called with the original file and must return the path to use when writing a modified version.
	ModifiedFileFn func(string) string

	Options struct {
		// Aspects is the set of configured injections to attempt.
		Aspects []aspect.Aspect
		// Dir is the working directory to use for resolving dependencies, etc... If blank, the current working directory is
		// used.
		Dir string
		// IncludeTests requests test files to be prepared for injection, too.
		IncludeTests bool
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
		// Explicitly disable toolexec for this, as if provided via $GOFLAGS, it
		// would be honored by the go/packages loader and that'd cause this call to
		// become a fork-bomb.
		BuildFlags: []string{"-toolexec="},
		Dir:        opts.Dir,
		Fset:       fileset,
		Mode: packages.NeedName |
			packages.NeedFiles |
			packages.NeedImports |
			packages.NeedTypes |
			packages.NeedTypesSizes |
			packages.NeedSyntax |
			packages.NeedTypesInfo,
		Tests: opts.IncludeTests,
	}
	if flags, err := goflags.Flags(); err == nil {
		// Honor any `-tags`  flags provided by the user, as these may affect what
		// is getting compiled or not.
		if tags, hasTags := flags.Get("-tags"); hasTags {
			cfg.BuildFlags = append(cfg.BuildFlags, fmt.Sprintf("-tags=%s", tags))
		}
	}

	var (
		pkgPath     string
		decorators  []*decorator.Decorator
		restorerMap map[string]string
	)
	if pkgs, err := decorator.Load(cfg, pkgDir); err != nil {
		return nil, err
	} else {
		decorators = make([]*decorator.Decorator, 0, len(pkgs))
		restorerMap = make(map[string]string, len(builtin.RestorerMap))
		for path, name := range builtin.RestorerMap {
			restorerMap[path] = name
		}

		for _, pkg := range pkgs {
			if len(pkg.Errors) > 0 {
				errs := make([]error, len(pkg.Errors))
				for i := range pkg.Errors {
					errs[i] = pkg.Errors[i]
				}
				return nil, errors.Join(errs...)
			}

			if pkgPath == "" {
				// The first non-blank package path is the "top level" one (the one we care about).
				pkgPath = pkg.PkgPath
			}

			for _, imp := range pkg.Imports {
				if imp.Name == "" {
					// Happens when there is an error while processing the import, typically inability to resolve the name due to
					// a typo or something. If we allow blank names in the map, the restorer just removes the qualifiers, which is
					// obviously undesirable.
					continue
				}
				restorerMap[imp.PkgPath] = imp.Name
			}

			// pkg.Decorator is nil in case the package in question does not include any go source file. This can be the case
			// when building test packages that do not include any non-test source file; in which case the "package under
			// test" is empty. This is because the loader returns three different entries when processing tests:
			// 1. The package under test (which may be empty)
			// 2. The test functions
			// 3. The test binary/main
			if pkg.Decorator != nil {
				decorators = append(decorators, pkg.Decorator)
			}
		}
	}

	if len(decorators) == 0 {
		return nil, errors.New("no decorators could be created")
	}

	return &Injector{
		fileset:    fileset,
		decorators: decorators,
		restorer:   decorator.NewRestorerWithImports(pkgPath, guess.WithMap(restorerMap)),
		opts:       opts,
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

	file, decorator, err := i.lookupDecoratedFile(filename)
	if err != nil {
		return res, err
	}

	ctx := typed.ContextWithValue(context.Background(), decorator)
	if res.Modified, res.References, err = i.inject(ctx, file, decorator, rootConfig); err != nil {
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

func (i *Injector) lookupDecoratedFile(filename string) (*dst.File, *decorator.Decorator, error) {
	stat, err := os.Stat(filename)
	if err != nil {
		return nil, nil, err
	}

	for _, dec := range i.decorators {
		for node, name := range dec.Filenames {
			if name == "" {
				// A bunch of synthetic nodes won't have a file name.
				continue
			}
			s, err := os.Stat(name)
			if err != nil {
				continue
			}
			if os.SameFile(stat, s) {
				return node, dec, nil
			}
		}
	}

	src, err := os.ReadFile(filename)
	if err != nil {
		return nil, nil, err
	}

	decorator := i.decorators[0]
	file, err := decorator.ParseFile(filename, src, parser.ParseComments)
	if err != nil {
		return nil, nil, err
	}

	return file, decorator, nil
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

	if mod && i.opts.PreserveLineInfo {
		i.addLineDirectives(file, decorator)
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
func (i *Injector) addLineDirectives(file *dst.File, decorator *decorator.Decorator) {
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
