// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package aspect

import (
	"context"
	"fmt"
	"os"
	"sort"

	"github.com/DataDog/dd-trace-go/v2/ddtrace/tracer"
	"github.com/DataDog/orchestrion/internal/jobserver/pkgs"
	"github.com/DataDog/orchestrion/internal/toolexec/archive"
	"github.com/DataDog/orchestrion/internal/toolexec/aspect/linkdeps"
	"github.com/DataDog/orchestrion/internal/toolexec/importcfg"
	"github.com/DataDog/orchestrion/internal/toolexec/proxy"
	"github.com/rs/zerolog"
)

func (w Weaver) OnLink(ctx context.Context, cmd *proxy.LinkCommand) (err error) {
	spanOptions := []tracer.StartSpanOption{tracer.ResourceName(w.ImportPath)}
	if w.Variant != "" {
		spanOptions = append(spanOptions, tracer.Tag("variant", w.Variant))
	}
	span, ctx := tracer.StartSpanFromContext(ctx, "Weaver.OnLink", spanOptions...)
	defer func() { span.Finish(tracer.WithError(err)) }()

	logContext := zerolog.Ctx(ctx).With().Str("phase", "link")
	if w.Variant != "" {
		logContext = logContext.Str("variant", w.Variant)
	}
	log := logContext.Logger()
	ctx = log.WithContext(ctx)

	reg, err := importcfg.ParseFile(ctx, cmd.Flags.ImportCfg)
	if err != nil {
		return fmt.Errorf("parsing %q: %w", cmd.Flags.ImportCfg, err)
	}

	testMain, _, err := cmd.TestMainInfo(ctx)
	if err != nil {
		return fmt.Errorf("reading test-main metadata: %w", err)
	}
	testVariantFor := testMain.Target
	if err := refreshTestMainPackageFiles(ctx, &testMain, &reg, cmd.WorkDir, resolveReversePackageFilesForTest); err != nil {
		return err
	}
	variantArchives := make(map[string]string, len(testMain.PackageFiles))
	changed := false
	for importPath, archive := range testMain.PackageFiles {
		variantArchives[importPath] = archive
		if reg.PackageFile[importPath] == archive {
			continue
		}
		reg.PackageFile[importPath] = archive
		changed = true
	}

	type archiveWork struct {
		importPath string
		archive    string
	}
	queue := make([]archiveWork, 0, len(reg.PackageFile))
	for importPath, archive := range reg.PackageFile {
		queue = append(queue, archiveWork{importPath: importPath, archive: archive})
	}
	less := func(i int, j int) bool {
		if queue[i].importPath == queue[j].importPath {
			return queue[i].archive < queue[j].archive
		}
		return queue[i].importPath < queue[j].importPath
	}
	sort.Slice(queue, less)
	processed := make(map[archiveWork]bool)
	resolveTestTargetProvenance := newTestTargetProvenanceResolver(ctx, testVariantFor, cmd.WorkDir)
	for len(queue) > 0 {
		item := queue[0]
		queue = queue[1:]
		if processed[item] {
			continue
		}
		processed[item] = true
		if current, found := reg.PackageFile[item.importPath]; found && current != item.archive {
			// A test-variant resolution replaced this archive after it was queued.
			// Its stale dependency metadata no longer describes the selected closure.
			continue
		}

		linkDeps, err := linkdeps.FromArchive(ctx, item.archive)
		if err != nil {
			return fmt.Errorf("reading %s from %q: %w", linkdeps.Filename, item.importPath, err)
		}
		log.Debug().Str("import-path", item.importPath).Str("archive", item.archive).Msg("Processing " + linkdeps.Filename + " dependencies")
		for _, depPath := range linkDeps.Dependencies() {
			kind := linkDeps.Kind(depPath)
			if depPath == testVariantFor && kind == linkdeps.ImportDependency &&
				(variantArchives[item.importPath] == item.archive || item.importPath == testVariantFor+".test") {
				continue
			}
			if arch, found := reg.PackageFile[depPath]; found && (testVariantFor == "" || depPath == testVariantFor) {
				var selected pkgs.ResolvedArchive
				if depPath == testVariantFor && kind == linkdeps.ImportDependency {
					selected, err = resolveTestTargetProvenance()
					if err != nil {
						return fmt.Errorf("resolving test target provenance for %q: %w", testVariantFor, err)
					}
				}
				if err := rejectSatisfiedSyntheticDependency(item.importPath, depPath, testVariantFor, kind, selected); err != nil {
					return err
				}
				log.Debug().Str("import-path", depPath).Str("archive", arch).Msg("Already satisfied " + linkdeps.Filename + " dependency")
				continue
			}

			log.Trace().Str("import-path", depPath).Msg("Resolving " + linkdeps.Filename + " dependency")
			deps, err := resolvePackageFilesForTest(ctx, depPath, testVariantFor, cmd.WorkDir)
			if err != nil {
				return fmt.Errorf("resolving %q: %w", depPath, err)
			}
			requiresRebuild := kind == linkdeps.ImportDependency
			if err := rejectResolvedSyntheticVariantDependency(item.importPath, depPath, testVariantFor, requiresRebuild, deps); err != nil {
				return err
			}
			updates, err := mergeResolvedArchives(&reg, deps, testVariantFor)
			if err != nil {
				return err
			}
			added := false
			for p, archive := range updates {
				log.Debug().Str("import-path", p).Str("archive", archive).Msg("Recording resolved " + linkdeps.Filename + " dependency")
				if deps[p].ForTest == testVariantFor {
					variantArchives[p] = archive
				}
				queue = append(queue, archiveWork{importPath: p, archive: archive})
				added = true
				changed = true
			}
			if added {
				sort.Slice(queue, less)
			}
		}
	}

	if !changed {
		return nil
	}

	log.Trace().Str("path", cmd.Flags.ImportCfg).Msg("Backing up original file")
	if err := os.Rename(cmd.Flags.ImportCfg, cmd.Flags.ImportCfg+".original"); err != nil {
		return fmt.Errorf("renaming %q: %w", cmd.Flags.ImportCfg, err)
	}
	log.Trace().Str("path", cmd.Flags.ImportCfg).Msg("Writing updated file")
	if err := reg.WriteFile(cmd.Flags.ImportCfg); err != nil {
		return fmt.Errorf("writing updated %q: %w", cmd.Flags.ImportCfg, err)
	}

	return nil
}

type reversePackageFilesResolver func(
	ctx context.Context,
	importPath string,
	testVariantFor string,
	authoritativeTarget string,
	workDir string,
) (pkgs.ResolveResponse, error)

func refreshTestMainPackageFiles(
	ctx context.Context,
	info *proxy.TestMainInfo,
	reg *importcfg.ImportConfig,
	workDir string,
	resolve reversePackageFilesResolver,
) error {
	if len(info.PackageFiles) == 0 {
		return nil
	}
	stale := false
	for importPath, filename := range info.PackageFiles {
		fingerprint, err := archive.CompatibilityFingerprint(filename)
		if err != nil || fingerprint != info.PackageFingerprints[importPath] {
			stale = true
			break
		}
	}
	if !stale {
		return nil
	}
	if len(info.ReverseRoots) == 0 {
		return fmt.Errorf("cached test-main metadata for %q references unavailable reverse variants and has no reconstruction roots", info.Target)
	}
	authoritative := reg.PackageFile[info.Target]
	if authoritative == "" {
		return fmt.Errorf("test-main import configuration is missing the authoritative package-under-test archive %q", info.Target)
	}
	refreshed := make(map[string]string)
	for _, root := range info.ReverseRoots {
		variants, err := resolve(ctx, root, info.Target, authoritative, workDir)
		if err != nil {
			return fmt.Errorf("reconstructing cached reverse test variant %q for %q: %w", root, info.Target, err)
		}
		for importPath, resolved := range variants {
			refreshed[importPath] = resolved.ExportFile
		}
	}
	selected := make(map[string]string, len(info.PackageFingerprints))
	for importPath, expected := range info.PackageFingerprints {
		filename := refreshed[importPath]
		if filename == "" {
			return fmt.Errorf("reconstructed reverse test variants for %q omitted %q", info.Target, importPath)
		}
		fingerprint, err := archive.CompatibilityFingerprint(filename)
		if err != nil {
			return fmt.Errorf("fingerprinting reconstructed reverse test variant %q: %w", importPath, err)
		}
		if fingerprint != expected {
			return fmt.Errorf("reconstructed reverse test variant %q has fingerprint %s, cached test main expects %s", importPath, fingerprint, expected)
		}
		selected[importPath] = filename
	}
	info.PackageFiles = selected
	return nil
}
