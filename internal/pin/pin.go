// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package pin

import (
	"bytes"
	"errors"
	"fmt"
	"go/parser"
	"go/token"
	goversion "go/version"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/DataDog/orchestrion/internal/goenv"
	"github.com/DataDog/orchestrion/internal/log"
	"github.com/DataDog/orchestrion/internal/version"
	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
	"golang.org/x/tools/go/packages"
)

const (
	orchestrionImportPath     = "github.com/DataDog/orchestrion"
	orchestrionInstrumentPath = orchestrionImportPath + "/instrument"
	orchestrionToolGo         = "orchestrion.tool.go"
	orchestrionDotYML         = "orchestrion.yml"
)

type Options struct {
	// Writer is the writer to send output of the command to.
	Writer io.Writer
	// ErrWriter is the writer to send error messages to.
	ErrWriter io.Writer

	// NoGenerate disables emitting a `//go:generate` directive (which is
	// otherwise emitted to facilitate automated upkeep of the contents of the
	// [orchestrionToolGo] file).
	NoGenerate bool
	// NoPrune disables removing unnecessary imports from the [orchestrionToolGo]
	// file. It will instead only print warnings about these.
	NoPrune bool
}

// parseOrchestrionToolGo reads the contents of the orchestrion tool file at the given path
// and returns the corresponding [*dst.File]
func parseOrchestrionToolGo(path string) (*dst.File, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %q: %w", path, err)
	}

	fset := token.NewFileSet()
	astFile, err := parser.ParseFile(fset, path, src, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parsing %q: %w", path, err)
	}
	dstFile, err := decorator.DecorateFile(fset, astFile)
	if err != nil {
		return nil, fmt.Errorf("decorating %q: %w", path, err)
	}

	return dstFile, nil
}

// defaultOrchestrionToolGo returns the default content of the orchestrion tool file when none is found.
func defaultOrchestrionToolGo() *dst.File {
	return &dst.File{
		Decs: dst.FileDecorations{
			NodeDecs: dst.NodeDecs{
				Start: dst.Decorations{
					"// This file was created by `orchestrion pin`, and is used to ensure the",
					"// `go.mod` file contains the necessary entries to ensure repeatable builds when",
					"// using `orchestrion`. It is also used to set up which integrations are enabled.",
					"\n",
					"//go:build tools",
					"\n",
				},
			},
		},
		Name: &dst.Ident{Name: "tools"},
	}
}

// PinOrchestrion applies or update the orchestrion pin file in the current
// working directory, according to the supplied [Options].
func PinOrchestrion(opts Options) error {
	goMod, err := goenv.GOMOD()
	if err != nil {
		return fmt.Errorf("getting GOMOD: %w", err)
	}

	toolFile := filepath.Join(goMod, "..", orchestrionToolGo)
	dstFile, err := parseOrchestrionToolGo(toolFile)
	if errors.Is(err, os.ErrNotExist) {
		log.Debugf("no %q file found, creating a new one", orchestrionToolGo)
		dstFile = defaultOrchestrionToolGo()
	}

	updateGoGenerateDirective(opts.NoGenerate, dstFile)

	importSet, err := updateToolFile(dstFile)
	if err != nil {
		return fmt.Errorf("updating %s file AST: %w", orchestrionToolGo, err)
	}

	if err := writeUpdated(toolFile, dstFile); err != nil {
		return fmt.Errorf("updating %q: %w", toolFile, err)
	}

	// Add the current version of orchestrion to the `go.mod` file.
	var edits []goModEdit
	curMod, err := parseGoMod(goMod)
	if err != nil {
		return fmt.Errorf("parsing %q: %w", goMod, err)
	}
	if goversion.Compare(fmt.Sprintf("go%s", curMod.Go), "go1.22.0") < 0 {
		edits = append(edits, goModVersion("1.22.0"))
	}
	if !curMod.requires(orchestrionImportPath) {
		edits = append(edits, goModRequire{Path: orchestrionImportPath, Version: version.Tag})
	}
	if err := runGoModEdit(goMod, edits...); err != nil {
		return fmt.Errorf("editing %q: %w", goMod, err)
	}

	pruned, err := pruneImports(importSet, opts)
	if err != nil {
		return fmt.Errorf("pruning imports from %q: %w", toolFile, err)
	}

	if pruned {
		// Run "go mod tidy" to ensure the `go.mod` file is up-to-date with detected dependencies.
		if err := runGoMod("tidy", goMod, nil); err != nil {
			return fmt.Errorf("running `go mod tidy`: %w", err)
		}
	}

	// Restore the previous toolchain directive if `go mod tidy` had the nerve to touch it...
	if err := runGoModEdit(goMod, curMod.Toolchain); err != nil {
		return fmt.Errorf("restoring toolchain directive: %w", err)
	}

	return nil
}

// updateToolFile updates the provided [*dst.File] according to the receiving
// [*Options], adding any new imports necessary. It returns the up-to-date
// [*importSet] for the file.
func updateToolFile(file *dst.File) (*importSet, error) {
	importSet := importSetFrom(file)

	if spec, isNew := importSet.Add(orchestrionImportPath); isNew {
		spec.Decs.Before = dst.NewLine
		spec.Decs.Start.Append(
			"// Ensures `orchestrion` is present in `go.mod` so that builds are repeatable.",
			"// Do not remove.",
		)
		spec.Decs.After = dst.NewLine
	}

	spec, isNew := importSet.Add(orchestrionInstrumentPath)
	if isNew {
		spec.Decs.Before = dst.NewLine
		spec.Decs.Start.Append(
			"// Provides integrations for essential `orchestrion` features. Most users",
			"// should not remove this integration.",
		)
		spec.Decs.After = dst.NewLine
	}
	spec.Decs.End.Replace("// integration")

	return importSet, nil
}

// updateGoGenerateDirective adds, updates, or removes the `//go:generate`
// directive from the [*dst.File] according to the receiving [*Options].
func updateGoGenerateDirective(noGenerate bool, file *dst.File) {
	const prefix = "//go:generate go run github.com/DataDog/orchestrion pin"

	newDirective := ""
	if !noGenerate {
		newDirective = prefix
		// TODO: Add additional CLI arguments here
	}

	found := false
	dst.Walk(
		dstNodeVisitor(func(node dst.Node) bool {
			switch node := node.(type) {
			case *dst.File, dst.Decl:
				decs := node.Decorations()
				for i, dec := range decs.Start {
					if dec != prefix && !strings.HasPrefix(dec, prefix+" ") {
						continue
					}
					found = true
					decs.Start[i] = newDirective
				}
				for i, dec := range decs.End {
					if dec != prefix && !strings.HasPrefix(dec, prefix+" ") {
						continue
					}
					found = true
					decs.End[i] = newDirective
				}
				return true
			default:
				return false
			}
		}),
		file,
	)

	if found || newDirective == "" {
		return
	}

	file.Decs.Start.Append("\n", newDirective, "\n")
}

// pruneImports removes unnecessary or invalid imports from the provided
// [*importSet]; unless the [*Options.NoPrune] field is true, in which case it
// only outputs a message informing the user about uncalled-for imports.
func pruneImports(importSet *importSet, opts Options) (bool, error) {
	pkgs, err := packages.Load(
		&packages.Config{
			BuildFlags: []string{"-toolexec="},
			Logf:       func(format string, args ...any) { log.Tracef(format+"\n", args...) },
			Mode:       packages.NeedName | packages.NeedFiles,
		},
		importSet.Except(orchestrionImportPath, orchestrionInstrumentPath)...,
	)
	if err != nil {
		return false, fmt.Errorf("pruneImports: %w", err)
	}

	var pruned bool
	for _, pkg := range pkgs {
		var someFile string
		for _, set := range [][]string{pkg.GoFiles, pkg.IgnoredFiles, pkg.OtherFiles} {
			if len(set) == 0 {
				continue
			}
			someFile = set[0]
		}
		// There is no compilation unit in this package, so it cannot have integrations.
		if someFile == "" {
			pruned = pruneImport(importSet, pkg.PkgPath, "the package contains no source files", opts) || pruned
			continue
		}
		integrationsFile := filepath.Join(someFile, "..", orchestrionDotYML)
		if _, err := os.Stat(integrationsFile); err != nil {
			if os.IsNotExist(err) {
				pruned = pruneImport(importSet, pkg.PkgPath, "there is no "+orchestrionDotYML+" file in this package", opts) || pruned
				continue
			}
		}
		importSet.Find(pkg.PkgPath).Decs.End.Replace("// integration")
	}

	return pruned, nil
}

// pruneImport prunes a single import from the supplied [*importSet], unless
// [*Options.NoPrune] is set, in which case it prints a warning using the
// provided `reason` message.
func pruneImport(importSet *importSet, path string, reason string, opts Options) bool {
	if opts.NoPrune {
		spec := importSet.Find(path)
		if spec == nil {
			// Nothing to do... already removed!²
			return false
		}

		_, _ = fmt.Fprintf(opts.Writer, "unnecessary import of %q: %v\n", path, reason)
		spec.Decs.End.Clear() // Remove the // integration comment.

		return false
	}

	if importSet.Remove(path) {
		_, _ = fmt.Fprintf(opts.Writer, "removing unnecessary import of %q: %v\n", path, reason)
	}
	return true
}

// writeUpdated writes the updated AST to the given file, using a temporary file
// to write the content before renaming it, to maximize atomicity of the update.
func writeUpdated(filename string, file *dst.File) error {
	var src bytes.Buffer
	if err := decorator.Fprint(&src, file); err != nil {
		return fmt.Errorf("formatting source code for %q: %w", filename, err)
	}

	tmpFile, err := os.CreateTemp(filepath.Dir(filename), filepath.Base(filename)+".*")
	if err != nil {
		return fmt.Errorf("creating temporary file for %q: %w", filename, err)
	}

	tmpFilename := tmpFile.Name()
	if _, err := io.Copy(tmpFile, &src); err != nil {
		return errors.Join(fmt.Errorf("writing to temporary file %q: %w", tmpFilename, err), tmpFile.Close())
	}

	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("closing %q: %w", tmpFilename, err)
	}

	if err := os.Rename(tmpFilename, filename); err != nil {
		return fmt.Errorf("renaming %q => %q: %w", tmpFilename, filename, err)
	}

	return nil
}

type dstNodeVisitor func(dst.Node) bool

func (v dstNodeVisitor) Visit(node dst.Node) dst.Visitor {
	if v(node) {
		return v
	}
	return nil
}
