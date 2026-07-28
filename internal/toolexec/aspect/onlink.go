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
	"github.com/DataDog/orchestrion/internal/toolexec/aspect/linkdeps"
	"github.com/DataDog/orchestrion/internal/toolexec/importcfg"
	"github.com/DataDog/orchestrion/internal/toolexec/proxy"
	"github.com/rs/zerolog"
)

func (w Weaver) OnLink(ctx context.Context, cmd *proxy.LinkCommand) (err error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "Weaver.OnLink",
		tracer.ResourceName(w.ImportPath),
	)
	defer func() { span.Finish(tracer.WithError(err)) }()

	log := zerolog.Ctx(ctx).With().Str("phase", "link").Logger()
	ctx = log.WithContext(ctx)

	reg, err := importcfg.ParseFile(ctx, cmd.Flags.ImportCfg)
	if err != nil {
		return fmt.Errorf("parsing %q: %w", cmd.Flags.ImportCfg, err)
	}

	testVariantFor, _, err := cmd.TestVariantFor(ctx)
	if err != nil {
		return fmt.Errorf("reading test-main metadata: %w", err)
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
	var changed bool
	for len(queue) > 0 {
		item := queue[0]
		queue = queue[1:]
		if processed[item] {
			continue
		}
		processed[item] = true

		linkDeps, err := linkdeps.FromArchive(ctx, item.archive)
		if err != nil {
			return fmt.Errorf("reading %s from %q: %w", linkdeps.Filename, item.importPath, err)
		}
		log.Debug().Str("import-path", item.importPath).Str("archive", item.archive).Msg("Processing " + linkdeps.Filename + " dependencies")
		for _, depPath := range linkDeps.Dependencies() {
			if arch, found := reg.PackageFile[depPath]; found && testVariantFor == "" {
				log.Debug().Str("import-path", depPath).Str("archive", arch).Msg("Already satisfied " + linkdeps.Filename + " dependency")
				continue
			}

			log.Trace().Str("import-path", depPath).Msg("Resolving " + linkdeps.Filename + " dependency")
			deps, err := resolvePackageFilesForTest(ctx, depPath, testVariantFor, cmd.WorkDir)
			if err != nil {
				return fmt.Errorf("resolving %q: %w", depPath, err)
			}
			parent := item.importPath
			if parent == testVariantFor {
				parent = ""
			}
			if err := rejectSyntheticVariantDependency(parent, depPath, testVariantFor, deps); err != nil {
				return err
			}
			updates, err := mergeResolvedArchives(&reg, deps, testVariantFor)
			if err != nil {
				return err
			}
			added := false
			for p, archive := range updates {
				log.Debug().Str("import-path", p).Str("archive", archive).Msg("Recording resolved " + linkdeps.Filename + " dependency")
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
