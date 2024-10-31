// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package pin

import (
	"bytes"
	"fmt"
	"go/parser"
	"go/token"
	goVersion "go/version"
	"io"
	"os"
	"os/exec"
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
	OrchestrionToolGo         = "orchestrion.tool.go"
	orchestrionDotYML         = "orchestrion.yml"
)

type Options struct {
	// Writer is the write to send output of the command to.
	Writer io.Writer
	// ErrWriter is the writer to send error messages to.
	ErrWriter io.Writer

	// NoGenerate disables emitting a `//go:generate` directive (which is
	// otherwise emitted to facilitate automated upkeep of the contents of the
	// [OrchestrionToolGo] file).
	NoGenerate bool
	// NoPrune disables removing unnecessary imports from the [OrchestrionToolGo]
	// file. It will instead only print warnings about these.
	NoPrune bool
}

func PinOrchestrion(opts Options) error {
	goMod, err := goenv.GOMOD()
	if err != nil {
		return fmt.Errorf("getting GOMOD: %w", err)
	}

	toolFile := filepath.Join(goMod, "..", OrchestrionToolGo)
	var dstFile *dst.File
	if src, err := os.ReadFile(toolFile); err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("reading %q: %w", toolFile, err)
		}
		dstFile = &dst.File{
			Decs: dst.FileDecorations{
				NodeDecs: dst.NodeDecs{
					Start: dst.Decorations{
						"// This file was created by `orchestrion pin`, and is used to ensure the",
						"// `go.mod` file contains the necessary entries to ensure repeatable builds when",
						"// using `orchestrion`. It is also used to set up which tracer integrations are",
						"// enabled.",
						"\n",
						"//go:build tools",
						"\n",
					},
				},
			},
			Name: &dst.Ident{Name: "tools"},
		}
	} else {
		fset := token.NewFileSet()
		astFile, err := parser.ParseFile(fset, toolFile, src, parser.ParseComments)
		if err != nil {
			return fmt.Errorf("parsing %q: %w", toolFile, err)
		}
		dstFile, err = decorator.DecorateFile(fset, astFile)
		if err != nil {
			return fmt.Errorf("decorating %q: %w", toolFile, err)
		}
	}

	if err := opts.updateToolFile(dstFile); err != nil {
		return fmt.Errorf("updating %s file AST: %w", OrchestrionToolGo, err)
	}

	var buf bytes.Buffer
	if err := decorator.Fprint(&buf, dstFile); err != nil {
		return fmt.Errorf("formatting %q: %w", toolFile, err)
	}

	// We write into a temporary file, and then rename it in place. This reduces the risk of
	// concurrent calls resulting in partial writes, etc...
	tmpFile, err := os.CreateTemp(filepath.Dir(toolFile), "orchestrion.tool.go.*")
	if err != nil {
		return fmt.Errorf("creating temporary %q: %w", tmpFile.Name(), err)
	}
	if _, err := io.Copy(tmpFile, &buf); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("writing to %q: %w", tmpFile.Name(), err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("closing %q: %w", tmpFile.Name(), err)
	}
	if err := os.Rename(tmpFile.Name(), toolFile); err != nil {
		return fmt.Errorf("renaming %q to %q: %w", tmpFile.Name(), toolFile, err)
	}

	// Add the current version of orchestrion to the `go.mod` file.
	var edits []string
	curMod, err := parse(goMod)
	if err != nil {
		return fmt.Errorf("parsing %q: %w", goMod, err)
	}
	if goVersion.Compare("go"+curMod.Go, "go1.22.0") < 0 {
		edits = append(edits, "-go=1.22.0")
	}
	if !curMod.requires(orchestrionImportPath) && !curMod.replaces(orchestrionImportPath) {
		edits = append(edits, "-require="+orchestrionImportPath+"@"+version.Tag)
	}
	if len(edits) > 0 {
		cmd := exec.Command("go", "mod", "edit")
		cmd.Args = append(cmd.Args, edits...)
		cmd.Env = append(os.Environ(), "GOTOOLCHAIN=local", "GOMOD="+goMod)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("running `go mod edit` to require %s@%s: %w", orchestrionImportPath, version.Tag, err)
		}
	}

	// Run "go mod tidy" to ensure the `go.mod` file is up-to-date with detected dependencies.
	cmd := exec.Command("go", "mod", "tidy")
	cmd.Env = append(os.Environ(), "GOTOOLCHAIN=local", "GOMOD="+goMod)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("running `go mod tidy`: %w", err)
	}

	// Restore the previous toolchain directive if `go mod tidy` had the nerve to touch it...
	cmd = exec.Command("go", "mod", "edit", "-toolchain="+curMod.Toolchain)
	cmd.Env = append(os.Environ(), "GOTOOLCHAIN=local", "GOMOD="+goMod)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("running `go mod edit` to reset toolchain: %w", err)
	}

	return nil
}

func (opts *Options) updateToolFile(file *dst.File) error {
	opts.updateGoGenerateDirective(file)

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

	return opts.pruneImports(importSet)
}

func (opts *Options) updateGoGenerateDirective(file *dst.File) {
	const prefix = "//go:generate orchestrion pin"

	var newDirective string
	if !opts.NoGenerate {
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
					decs.Start[i] = newDirective
					found = true
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

func (opts *Options) pruneImports(importSet *importSet) error {
	pkgs, err := packages.Load(
		&packages.Config{
			BuildFlags: []string{"-toolexec="},
			Logf:       func(format string, args ...any) { log.Tracef(format+"\n", args...) },
			Mode:       packages.NeedName | packages.NeedFiles,
		},
		importSet.Except(orchestrionImportPath, orchestrionInstrumentPath)...,
	)
	if err != nil {
		return fmt.Errorf("pruneImports: %w", err)
	}

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
			opts.pruneImport(importSet, pkg.PkgPath, "the package contains no Go source files")
			continue
		}
		integrationsFile := filepath.Join(someFile, "..", orchestrionDotYML)
		if _, err := os.Stat(integrationsFile); err != nil {
			if os.IsNotExist(err) {
				opts.pruneImport(importSet, pkg.PkgPath, "there is no "+orchestrionDotYML+" file in this package")
				continue
			}
		}
		importSet.Find(pkg.PkgPath).Decs.End.Replace("// integration")
	}

	return nil
}

func (opts *Options) pruneImport(importSet *importSet, path string, reason string) {
	if opts.NoPrune {
		spec := importSet.Find(path)
		if spec == nil {
			// Nothing to do... already removed!
			return
		}

		_, _ = fmt.Fprintf(opts.Writer, "unnecessary import of %q: %v\n", path, reason)
		spec.Decs.End.Clear() // Remove the // integration comment.

		return
	}

	if importSet.Remove(path) {
		_, _ = fmt.Fprintf(opts.Writer, "removing unnecessary import of %q: %v\n", path, reason)
	}
}

type dstNodeVisitor func(dst.Node) bool

func (v dstNodeVisitor) Visit(node dst.Node) dst.Visitor {
	if v(node) {
		return v
	}
	return nil
}
