// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

// Package injector provides a facility to inject code into go programs, either
// in source (intended to be checked in by the user) or at compilation time
// (via `-toolexec`).
package injector

import (
	gocontext "context"
	"errors"
	"fmt"
	"go/importer"
	"go/token"
	"go/types"
	"sync"

	"github.com/DataDog/dd-trace-go/v2/ddtrace/tracer"
	"github.com/DataDog/orchestrion/internal/injector/aspect"
	"github.com/DataDog/orchestrion/internal/injector/aspect/advice"
	"github.com/DataDog/orchestrion/internal/injector/aspect/context"
	"github.com/DataDog/orchestrion/internal/injector/aspect/join"
	"github.com/DataDog/orchestrion/internal/injector/parse"
	"github.com/DataDog/orchestrion/internal/injector/typed"
	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
	"github.com/dave/dst/decorator/resolver"
	"github.com/dave/dst/decorator/resolver/gotypes"
	"github.com/dave/dst/dstutil"
	"github.com/rs/zerolog"
)

type (
	// Injector injects go code into a specific Go package.
	Injector struct {
		// ImportPath is the import path of the package that will be injected.
		ImportPath string
		// Name is the name of the package that will be injected. If blank, it will be determined from parsing source files.
		Name string
		// GoVersion is the go runtime version required by this package. If blank, no go runtime compatibility will be
		// asserted.
		GoVersion string
		// TestMain must be set to true when injecting into the generated test main package.
		TestMain bool

		// ImportMap is a map of import paths to their respective .a archive file. Without transitive dependencies
		ImportMap map[string]string

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

	parameters struct {
		Decorator *decorator.Decorator
		File      *dst.File
		TypeInfo  types.Info
		Aspects   []*aspect.Aspect
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
func (i *Injector) InjectFiles(ctx gocontext.Context, files []string, aspects []*aspect.Aspect) (_ map[string]InjectedFile, _ context.GoLangVersion, err error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "InjectFiles",
		tracer.ServiceName("github.com/DataDog/orchestrion/internal/injector"),
		tracer.ResourceName(i.ImportPath),
	)
	defer func() { span.Finish(tracer.WithError(err)) }()

	if err := i.validate(); err != nil {
		return nil, context.GoLangVersion{}, err
	}

	log := zerolog.Ctx(ctx)
	aspects = i.packageFilterAspects(aspects)

	if len(aspects) == 0 {
		log.Debug().Str("import-path", i.ImportPath).Msg("No aspects match this package after import filtering")
		return nil, context.GoLangVersion{}, nil
	}

	fset := token.NewFileSet()
	parser := parse.NewParser(fset, len(files))
	parsedFiles, err := parser.ParseFiles(ctx, files, aspects)
	if err != nil {
		return nil, context.GoLangVersion{}, err
	}

	if len(parsedFiles) == 0 {
		log.Debug().Str("import-path", i.ImportPath).Msg("No files to inject in package after filtering on imports and files")
		return nil, context.GoLangVersion{}, nil
	}

	// Check if any surviving aspect needs the expensive Types map (only
	// *implements join points use it). No production dd-trace-go aspects use
	// these, so this is typically false.
	needTypesMap := false
	for _, f := range parsedFiles {
		if join.NeedsTypesMap(pointsOf(f.Aspects)) {
			needTypesMap = true
			break
		}
	}
	typeInfo, err := i.typeCheck(ctx, fset, parsedFiles, needTypesMap)
	if errors.Is(err, typeCheckingError{}) {
		// We don't want to fail here on type-checking errors... Instead do nothing and let the standard
		// go compiler/toolchain surface the error to the user in a canonical way.
		log.Warn().Str("import-path", i.ImportPath).Err(err).Msg("Skipping injectrion due to type checking error")
		return nil, context.GoLangVersion{}, nil
	} else if err != nil {
		return nil, context.GoLangVersion{}, err
	}

	var (
		wg           sync.WaitGroup
		errs         []error
		errsMu       sync.Mutex
		result       = make(map[string]InjectedFile, len(parsedFiles))
		resultGoLang context.GoLangVersion
		resultMu     sync.Mutex
	)

	// Only decorate and transform files that have matching aspects. Files
	// with no aspects were parsed solely for type checking and don't need
	// the expensive DecorateFile + AST walk. In a typical package, only
	// 1-2 files out of 10+ have matching aspects, so this skips ~80-90%
	// of DecorateFile calls (which dominate CPU time via go/token position
	// lookups and RWMutex contention on the shared FileSet).
	var filesToInject []parse.File
	for _, f := range parsedFiles {
		if len(f.Aspects) > 0 {
			filesToInject = append(filesToInject, f)
		}
	}

	wg.Add(len(filesToInject))
	for _, parsedFile := range filesToInject {
		go func(parsedFile parse.File) {
			defer wg.Done()

			decorator := decorator.NewDecoratorWithImports(fset, i.ImportPath, gotypes.New(typeInfo.Uses))
			dstFile, err := decorator.DecorateFile(parsedFile.AstFile)
			if err != nil {
				errsMu.Lock()
				defer errsMu.Unlock()
				errs = append(errs, err)
				return
			}

			res, err := i.injectFile(ctx, decorator, dstFile, typeInfo, parsedFile.Aspects)
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
			result[parsedFile.Name] = res.InjectedFile
			resultGoLang.SetAtLeast(res.GoLang)
		}(parsedFile)
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

// pointsOf extracts join points from a list of aspects.
func pointsOf(aspects []*aspect.Aspect) []join.Point {
	points := make([]join.Point, len(aspects))
	for i, a := range aspects {
		points[i] = a.JoinPoint
	}
	return points
}

// injectFile injects code in the specified file. This method can be called concurrently by multiple goroutines,
// as is guarded by a sync.Mutex.
func (i *Injector) injectFile(ctx gocontext.Context, decorator *decorator.Decorator, file *dst.File, typeInfo types.Info, aspects []*aspect.Aspect) (result, error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "Injector.injectFile",
		tracer.ResourceName(decorator.Filenames[file]),
	)
	defer span.Finish()

	result, err := i.applyAspects(ctx, parameters{
		Decorator: decorator,
		File:      file,
		TypeInfo:  typeInfo,
		Aspects:   aspects,
	})
	if err != nil {
		return result, fmt.Errorf("%q: %w", result.Filename, err)
	}

	if result.Modified {
		span.SetTag("modified", true)

		result.Filename, err = i.writeModifiedFile(ctx, decorator, file)
		if err != nil {
			return result, err
		}
	}

	return result, nil
}

func (i *Injector) applyAspects(ctx gocontext.Context, params parameters) (result, error) {
	var (
		chain      *context.NodeChain
		modified   bool
		references = typed.NewReferenceMap(params.Decorator.Ast.Nodes, params.TypeInfo.Scopes)
		err        error
	)

	pre := func(csor *dstutil.Cursor) bool {
		if err != nil || csor.Node() == nil || isIgnored(ctx, csor.Node()) {
			return false
		}

		root := chain == nil
		chain = chain.Child(csor)
		if root {
			chain.SetConfig(i.RootConfig)
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
		ctx := chain.Context(ctx, context.ContextArgs{
			Cursor:       csor,
			ImportPath:   params.Decorator.Path,
			File:         params.File,
			RefMap:       &references,
			SourceParser: params.Decorator,
			MinGoLang:    &minGoLang,
			TestMain:     i.TestMain,
			TypeInfo:     params.TypeInfo,
			NodeMap:      params.Decorator.Ast.Nodes,
		})
		defer ctx.Release()

		changed, err = injectNode(ctx, params.Aspects)
		modified = modified || changed

		return err == nil
	}

	dstutil.Apply(params.File, pre, post)
	if err != nil {
		return result{}, err
	}

	// We only inject synthetic imports here because it may offset declarations by one position in
	// case a new import declaration is necessary, which causes dstutil.Apply to re-traverse the
	// current declaration.
	if references.AddSyntheticImports(params.File) {
		modified = true
	}

	return result{
		InjectedFile: InjectedFile{
			References: references,
			Filename:   params.Decorator.Filenames[params.File],
		},
		Modified: modified,
		GoLang:   minGoLang,
	}, nil
}

// injectNode assesses all configured aspects against the current node, and performs any AST
// transformations. It returns whether the AST was indeed modified. In case of an error, the
// injector aborts immediately and returns the error.
func injectNode(ctx context.AdviceContext, aspects []*aspect.Aspect) (mod bool, err error) {
	var orderedAdvice []*advice.OrderedAdvice
	var index int
	for _, inj := range aspects {
		if !inj.JoinPoint.Matches(ctx) {
			continue
		}

		for _, adv := range inj.Advice {
			orderedAdvice = append(orderedAdvice, advice.NewOrderedAdvice(inj.ID, adv, index))
			index++
		}
	}

	if len(orderedAdvice) == 0 {
		return false, nil
	}

	advice.Sort(orderedAdvice)
	for _, act := range orderedAdvice {
		var changed bool
		changed, err := act.Apply(ctx)
		mod = mod || changed
		if err != nil {
			return mod, fmt.Errorf("%q[%d]: %w", act.AspectID, act.Index, err)
		}
	}
	return mod, nil
}
