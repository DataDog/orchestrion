// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

// Package injector provides a facility to inject code into go programs, either
// in source (intended to be checked in by the user) or at compilation time
// (via `-toolexec`).
package injector

import (
	"errors"
	"fmt"
	"go/ast"
	"go/importer"
	"go/token"
	"sync"

	"github.com/DataDog/orchestrion/internal/injector/aspect"
	"github.com/DataDog/orchestrion/internal/injector/aspect/context"
	"github.com/DataDog/orchestrion/internal/injector/typed"
	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
	"github.com/dave/dst/decorator/resolver"
	"github.com/dave/dst/decorator/resolver/gotypes"
	"github.com/dave/dst/dstutil"
)

type (
	// Injector injects go code into a specific Go package.
	Injector struct {
		// Aspects is the set of configured aspects to use.
		Aspects []*aspect.Aspect

		// ImportPath is the import path of the package that will be injected.
		ImportPath string
		// Name is the name of the package that will be injected. If blank, it will be determined from parsing source files.
		Name string
		// GoVersion is the go runtime version required by this package. If blank, no go runtime compatibility will be
		// asserted.
		GoVersion string

		// ModifiedFile is called to determine the output file name for a modified file. If nil, the input file is modified
		// in-place.
		ModifiedFile func(string) string
		// Lookup is a function that resolves and imported package's archive file.
		Lookup importer.Lookup
		// RootConfig is the root configuration value to use.
		RootConfig map[string]string

		// restorerResolver is used to restore modified files. It's created on-demand then re-used.
		restorerResolver resolver.RestorerResolver
	}

	// InjectedFile contains information about a modified file. It can be used to update compilation instructions.
	InjectedFile struct {
		// References holds new references created while injecting the package, if any.
		References typed.ReferenceMap
		// Filename is the name of the file that needs to be compiled in place of the original one. It may be identical to
		// the input file if the Injector.ModifiedFile function is nil or returns identity.
		Filename string
	}

	result struct {
		InjectedFile
		Modified bool
		GoLang   context.GoLangVersion
	}
)

// InjectFiles performs injections on the specified files. All provided file paths must belong to the import path set on
// the receiving Injector. The method returns a map that associates the original source file path to the modified file
// information. It does not contain entries for unmodified files.
func (i *Injector) InjectFiles(files []string) (map[string]InjectedFile, context.GoLangVersion, error) {
	if err := i.validate(); err != nil {
		return nil, context.GoLangVersion{}, err
	}

	fset := token.NewFileSet()
	astFiles, err := parseFiles(fset, files)
	if err != nil {
		return nil, context.GoLangVersion{}, err
	}

	uses, err := i.typeCheck(fset, astFiles)
	if err != nil {
		return nil, context.GoLangVersion{}, err
	}

	var (
		wg           sync.WaitGroup
		errs         []error
		errsMu       sync.Mutex
		result       = make(map[string]InjectedFile, len(astFiles))
		resultGoLang context.GoLangVersion
		resultMu     sync.Mutex
	)

	wg.Add(len(astFiles))
	for idx, astFile := range astFiles {
		go func(idx int, astFile *ast.File) {
			defer wg.Done()

			decorator := decorator.NewDecoratorWithImports(fset, i.ImportPath, gotypes.New(uses))
			dstFile, err := decorator.DecorateFile(astFile)
			if err != nil {
				errsMu.Lock()
				defer errsMu.Unlock()
				errs = append(errs, err)
				return
			}

			res, err := i.injectFile(decorator, dstFile)
			if err != nil {
				errsMu.Lock()
				defer errsMu.Unlock()
				errs = append(errs, err)
				return
			}

			if !res.Modified {
				return
			}

			resultMu.Lock()
			defer resultMu.Unlock()
			result[files[idx]] = res.InjectedFile
			resultGoLang.SetAtLeast(res.GoLang)
		}(idx, astFile)
	}
	wg.Wait()

	return result, resultGoLang, errors.Join(errs...)
}

func (i *Injector) validate() error {
	var err error
	if i.ImportPath == "" {
		err = errors.Join(err, fmt.Errorf("invalid %T: missing ImportPath", i))
	}
	if i.Lookup == nil {
		err = errors.Join(err, fmt.Errorf("invalid %T: missing Lookup", i))
	}

	// Initialize the restorerResolver field, too...
	i.restorerResolver = &lookupResolver{lookup: i.Lookup}

	return err
}

// injectFile injects code in the specified file. This method can be called concurrently by multiple goroutines,
// as is guarded by a sync.Mutex.
func (i *Injector) injectFile(decorator *decorator.Decorator, file *dst.File) (result, error) {
	result := result{InjectedFile: InjectedFile{Filename: decorator.Filenames[file]}}

	var err error
	result.Modified, result.References, result.GoLang, err = i.applyAspects(decorator, file, i.RootConfig)
	if err != nil {
		return result, err
	}

	if result.Modified {
		result.Filename, err = i.writeModifiedFile(decorator, file)
		if err != nil {
			return result, err
		}
	}

	return result, nil
}

func (i *Injector) applyAspects(decorator *decorator.Decorator, file *dst.File, rootConfig map[string]string) (bool, typed.ReferenceMap, context.GoLangVersion, error) {
	var (
		chain      *context.NodeChain
		modified   bool
		references typed.ReferenceMap
		err        error
	)

	pre := func(csor *dstutil.Cursor) bool {
		if err != nil || csor.Node() == nil || isIgnored(csor.Node()) {
			return false
		}
		root := chain == nil
		chain = chain.Child(csor)
		if root {
			chain.SetConfig(rootConfig)
		}
		return true
	}

	var minGoLang context.GoLangVersion
	post := func(csor *dstutil.Cursor) bool {
		// Pop the ancestry stack now that we're done with this node.
		defer func() {
			old := chain
			chain = chain.Parent()
			old.Release()
		}()

		var changed bool
		ctx := chain.Context(context.ContextArgs{
			Cursor:       csor,
			ImportPath:   decorator.Path,
			File:         file,
			RefMap:       &references,
			SourceParser: decorator,
			MinGoLang:    &minGoLang,
		})
		defer ctx.Release()
		changed, err = i.injectNode(ctx)
		modified = modified || changed

		return err == nil
	}

	dstutil.Apply(file, pre, post)

	// We only inject synthetic imports here because it may offset declarations by one position in
	// case a new import declaration is necessary, which causes dstutil.Apply to re-traverse the
	// current declaration.
	if references.AddSyntheticImports(file) {
		modified = true
	}

	return modified, references, minGoLang, err
}

// injectNode assesses all configured aspects agaisnt the current node, and performs any AST
// transformations. It returns whether the AST was indeed modified. In case of an error, the
// injector aborts immediately and returns the error.
func (i *Injector) injectNode(ctx context.AdviceContext) (mod bool, err error) {
	for _, inj := range i.Aspects {
		if !inj.JoinPoint.Matches(ctx) {
			continue
		}
		for _, act := range inj.Advice {
			var changed bool
			changed, err = act.Apply(ctx)
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
