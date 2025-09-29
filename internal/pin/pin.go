// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package pin

import (
	"bytes"
	"context"
	"crypto/sha512"
	"encoding/base64"
	"errors"
	"fmt"
	"go/parser"
	"go/token"
	goversion "go/version"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/DataDog/orchestrion/internal/filelock"
	"github.com/DataDog/orchestrion/internal/goenv"
	"github.com/DataDog/orchestrion/internal/injector/config"
	"github.com/DataDog/orchestrion/internal/version"
	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
	"github.com/rs/zerolog"
	"golang.org/x/mod/semver"
	"golang.org/x/tools/go/packages"
)

const (
	orchestrionImportPath = "github.com/DataDog/orchestrion"
	datadogTracerV1       = "gopkg.in/DataDog/dd-trace-go.v1"
	datadogTracerV2       = "github.com/DataDog/dd-trace-go/v2"
	datadogTracerV2All    = "github.com/DataDog/dd-trace-go/orchestrion/all/v2"
)

var orchestrionShippedVersions = map[string]string{
	datadogTracerV2:    "latest",
	datadogTracerV2All: "latest",
}

type Options struct {
	// Writer is the writer to send output of the command to. Defaults to
	// [os.Stdout].
	Writer io.Writer
	// ErrWriter is the writer to send error messages to. Defaults to [os.Stderr].
	ErrWriter io.Writer

	// Validate checks the contents of all [orchestrionDotYML] files encountered
	// during the pinning process, ensuring they are valid according to the JSON
	// schema specification.
	Validate bool
	// NoGenerate disables emitting a `//go:generate` directive (which is
	// otherwise emitted to facilitate automated upkeep of the contents of the
	// [orchestrionToolGo] file).
	NoGenerate bool
	// NoPrune disables removing unnecessary imports from the [orchestrionToolGo]
	// file. It will instead only print warnings about these.
	NoPrune bool
}

// PinOrchestrion applies or update the orchestrion pin file in the current
// working directory, according to the supplied [Options].
func PinOrchestrion(ctx context.Context, opts Options) error {
	// Ensure we have an [Options.Writer] and [Options.ErrWriter] set.
	if opts.Writer == nil {
		opts.Writer = os.Stdout
	}
	if opts.ErrWriter == nil {
		opts.ErrWriter = os.Stderr
	}

	log := zerolog.Ctx(ctx)

	goMod, err := goenv.GOMOD("")
	if err != nil {
		return fmt.Errorf("getting GOMOD: %w", err)
	}

	// Acquire an advisory lock on the `go.mod` file, so that in `-toolexec` mode,
	// multiple attempts to auto-pin don't try to modify the files at the same
	// time. The `go mod tidy` command takes an advisory write-lock on `go.mod`,
	// so we are using a separate file under [os.TempDir] to avoid deadlocking.
	sha := sha512.Sum512([]byte(goMod))
	flockname := filepath.Join(os.TempDir(), "orchestrion-pin_"+base64.URLEncoding.EncodeToString(sha[:])+"_go.mod.lock")
	flock := filelock.MutexAt(flockname)
	if err := flock.Lock(ctx); err != nil {
		return fmt.Errorf("failed to acquire lock on %q: %w", goMod, err)
	}
	defer func() {
		if err := flock.Unlock(ctx); err != nil {
			log.Error().Err(err).Str("lock-file", goMod).Msg("Failed to release file lock")
		}
	}()

	toolFile := filepath.Join(goMod, "..", config.FilenameOrchestrionToolGo)
	dstFile, err := parseOrchestrionToolGo(toolFile)
	if errors.Is(err, os.ErrNotExist) {
		log.Debug().Msg("no " + config.FilenameOrchestrionToolGo + " file found, creating a new one")
		dstFile = defaultOrchestrionToolGo()
		err = nil
	}

	if err != nil {
		return err
	}

	updateGoGenerateDirective(opts, dstFile)

	importSet, err := updateToolFile(dstFile)
	if err != nil {
		return fmt.Errorf("updating %s file AST: %w", config.FilenameOrchestrionToolGo, err)
	}

	if err := writeUpdated(toolFile, dstFile); err != nil {
		return fmt.Errorf("updating %q: %w", toolFile, err)
	}

	// Add the current version of orchestrion to the `go.mod` file.
	var edits []goModEdit
	curMod, err := parseGoMod(ctx, goMod)
	if err != nil {
		return fmt.Errorf("parsing %q: %w", goMod, err)
	}

	if ver, found := curMod.requires(datadogTracerV1); found && semver.Compare(ver, "v1.74.0") < 0 {
		if err := runGoGet(ctx, goMod, datadogTracerV1+"@latest"); err != nil {
			return fmt.Errorf("go get "+datadogTracerV1+"@latest: %w", err)
		}
	}

	ver, found := curMod.requires(datadogTracerV2)
	targetVersion := resolveModVersion(datadogTracerV2, ver)
	log.Info().Str("current", ver).Str("target", targetVersion).Msg("Installing or upgrading " + datadogTracerV2 + " (via " + datadogTracerV2All + ")")
	if found || semver.Compare(ver, "v2.1.0") < 0 {
		// We install/upgrade the `orchestrion/all/v2` module as it includes all interesting contribs in its dependency
		// closure, so we don't have to manually verify all of them. The `go mod tidy` later will clean up if needed.
		target := datadogTracerV2All + "@" + targetVersion
		if err := runGoGet(ctx, goMod, target); err != nil {
			return fmt.Errorf("go get "+target+": %w", err)
		}
	}
	if targetVersion != "latest" {
		edits = append(edits, goModRequire{Path: datadogTracerV2, Version: targetVersion})
	}

	if goversion.Compare(fmt.Sprintf("go%s", curMod.Go), "go1.23.0") < 0 {
		log.Info().Msg("Setting go directive to 1.23.0")
		edits = append(edits, goModVersion("1.23.0"))
	}

	if ver, found := curMod.requires(orchestrionImportPath); !found || semver.Compare(ver, version.Tag()) < 0 {
		log.Info().Str("current", ver).Msg("Adding/updating require entry for " + orchestrionImportPath)
		version, _, _ := strings.Cut(version.Tag(), "+")
		edits = append(edits, goModRequire{Path: orchestrionImportPath, Version: version})
	}

	if err := runGoModEdit(ctx, goMod, edits...); err != nil {
		return fmt.Errorf("editing %q: %w", goMod, err)
	}

	pruned, err := pruneImports(ctx, importSet, opts)
	if err != nil {
		return fmt.Errorf("pruning imports from %q: %w", toolFile, err)
	}

	if pruned {
		// Run "go mod tidy" to ensure the `go.mod` file is up-to-date with detected dependencies.
		if err := runGoMod(ctx, "tidy", goMod, nil); err != nil {
			return fmt.Errorf("running `go mod tidy`: %w", err)
		}
	}

	// Restore the previous toolchain directive if `go mod tidy` had the nerve to touch it...
	if err := runGoModEdit(ctx, goMod, curMod.Toolchain); err != nil {
		return fmt.Errorf("restoring toolchain directive: %w", err)
	}

	return nil
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

// updateToolFile updates the provided [*dst.File] according to the receiving
// [*Options], adding any new imports necessary. It returns the up-to-date
// [*importSet] for the file.
func updateToolFile(file *dst.File) (*importSet, error) {
	importSet := importSetFrom(file)

	spec, isNew := importSet.Add(orchestrionImportPath)
	if isNew {
		spec.Decs.Before = dst.NewLine
		spec.Decs.Start.Append(
			"// Ensures `orchestrion` is present in `go.mod` so that builds are repeatable.",
			"// Do not remove.",
		)
		spec.Decs.After = dst.EmptyLine
	}
	spec.Decs.End.Replace("// integration")

	spec, _ = importSet.Add(datadogTracerV2All)
	spec.Decs.Before = dst.EmptyLine
	spec.Decs.End.Replace("// integration")

	// We auto-imported from dd-trace-go, so we can remove the legacy `/instrument` import if present.
	importSet.Remove(orchestrionImportPath + "/instrument")

	// We instrument natively with V2, so we no longer need to import the legacy V1 entry point.
	importSet.Remove(datadogTracerV1)

	return importSet, nil
}

// updateGoGenerateDirective adds, updates, or removes the `//go:generate`
// directive from the [*dst.File] according to the receiving [*Options].
func updateGoGenerateDirective(opts Options, file *dst.File) {
	const prefix = "//go:generate go run github.com/DataDog/orchestrion pin"

	newDirective := ""
	if !opts.NoGenerate {
		newDirective = prefix + " -generate"
		if opts.NoPrune {
			newDirective += " -prune=false"
		}
		if opts.Validate {
			newDirective += " -validate"
		}
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
func pruneImports(ctx context.Context, importSet *importSet, opts Options) (bool, error) {
	importPaths := importSet.Except(orchestrionImportPath)
	if len(importPaths) == 0 {
		// Nothing to do!
		return false, nil
	}

	log := zerolog.Ctx(ctx)
	pkgs, err := packages.Load(
		&packages.Config{
			BuildFlags: []string{"-toolexec="},
			Logf:       func(format string, args ...any) { log.Trace().Str("operation", "packages.Load").Msgf(format, args...) },
			Mode:       packages.NeedName | packages.NeedFiles,
		},
		importPaths...,
	)
	if err != nil {
		return false, fmt.Errorf("pruneImports: %w", err)
	}

	var pruned bool
	for _, pkg := range pkgs {
		hasConfig, err := config.HasConfig(ctx, nil, pkg, opts.Validate)
		if err != nil {
			pruned = pruneImport(importSet, pkg.PkgPath, err.Error(), opts) || pruned
			continue
		}
		if !hasConfig {
			pruned = pruneImport(importSet, pkg.PkgPath, "there is no "+config.FilenameOrchestrionYML+" nor "+config.FilenameOrchestrionToolGo+" file in this package", opts) || pruned
			continue
		}
		decl := importSet.Find(pkg.PkgPath)
		decl.Decs.End.Replace("// integration")
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

func resolveModVersion(modPath string, currentVersion string) string {
	shippedVersion := orchestrionShippedVersions[modPath]
	if shippedVersion == "latest" {
		return "latest"
	}

	if currentVersion == "" || semver.Compare(currentVersion, shippedVersion) < 0 {
		return shippedVersion
	}

	return currentVersion
}

func resolveDependencyVersion(modDir string, dependency string) (string, error) {
	goMod, err := goenv.GOMOD(modDir)
	if err != nil {
		return "", fmt.Errorf("getting GOMOD: %w", err)
	}
	mod, err := parseGoMod(context.Background(), goMod)
	if err != nil {
		return "", fmt.Errorf("parsing %q: %w", goMod, err)
	}
	ver, ok := mod.requires(dependency)
	if !ok {
		return "", fmt.Errorf("failed to find %q in %q", dependency, goMod)
	}
	return ver, nil
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

func init() {
	_, thisFile, _, _ := runtime.Caller(0)
	orchestrionSrcDir := filepath.Join(thisFile, "..", "..", "..")
	// The version of dd-trace-go that shipped with the current version of orchestrion.
	// We use this to determine if we need to upgrade dd-trace-go when pinning.
	orchestrionRoot := orchestrionSrcDir
	ver, err := resolveDependencyVersion(orchestrionRoot, datadogTracerV2)
	if err != nil {
		panic(fmt.Errorf("resolving %s version in %q: %w", datadogTracerV2, orchestrionRoot, err))
	}
	orchestrionShippedVersions[datadogTracerV2] = ver
	instrumentationRoot := filepath.Join(orchestrionSrcDir, "instrument")
	ver, err = resolveDependencyVersion(instrumentationRoot, datadogTracerV2All)
	if err != nil {
		panic(fmt.Errorf("resolving %s version in %q: %w", datadogTracerV2All, instrumentationRoot, err))
	}
	orchestrionShippedVersions[datadogTracerV2All] = ver
}
