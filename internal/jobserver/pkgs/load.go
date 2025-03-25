// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package pkgs

import (
	"context"
	"fmt"

	"github.com/DataDog/dd-trace-go/v2/ddtrace/tracer"
	"github.com/DataDog/orchestrion/internal/goflags"
	"github.com/rs/zerolog"
	"golang.org/x/tools/go/packages"
)

type (
	// LoadRequest is a request to load packages relative to a specific directory. It only loads the
	// packages' names and (source) files, not their dependencies or export file. The result is cached
	// for a given Dir+Pattern pair. Each pattern is loaded individually (so that they can be cached
	// independently).
	LoadRequest struct {
		Dir      string   `json:"dir"`      // The directory to resolve from (usually where `go.mod` is)
		Patterns []string `json:"patterns"` // Package pattern to resolve
	}
	// LoadResponse is the response to a [LoadRequest]. It contains the packages that were loaded.
	LoadResponse []*packages.Package
)

// packageLoader can be used as a [config.PackageLoader] implementation.
func (s *service) packageLoader(ctx context.Context, dir string, patterns ...string) ([]*packages.Package, error) {
	return s.load(ctx, LoadRequest{Dir: dir, Patterns: patterns})
}

func (LoadRequest) Subject() string         { return loadSubject }
func (LoadRequest) ResponseIs(LoadResponse) {}
func (r LoadRequest) ForeachSpanTag(set func(key string, value any)) {
	set("request.dir", r.Dir)
	set("request.patterns", r.Patterns)
}

func (s *service) load(ctx context.Context, req LoadRequest) (LoadResponse, error) {
	resp := make(LoadResponse, len(req.Patterns))

	log := zerolog.Ctx(ctx)

	for idx, pattern := range req.Patterns {
		var err error
		resp[idx], err = s.loaded.Load(fmt.Sprintf("%s\u0000%s", req.Dir, pattern), func() (_ *packages.Package, err error) {
			span, ctx := tracer.StartSpanFromContext(ctx, "Load",
				tracer.ServiceName("golang.org/x/tools/go/packages"),
				tracer.ResourceName(pattern),
			)
			defer func() { span.Finish(tracer.WithError(err)) }()

			goFlags, err := goflags.Flags(ctx)
			if err != nil {
				log.Warn().Err(err).Msg("Failed to obtain go build flags")
			}
			goFlags = goFlags.Except(
				"-a",        // Re-building everything here would be VERY expensive, as we'd re-build a lot of stuff multiple times
				"-toolexec", // We'll override `-toolexec` later with `orchestrion toolexec`, no need to pass multiple times...
			)

			cfg := &packages.Config{
				Context:    ctx,
				Dir:        req.Dir,
				Mode:       packages.NeedName | packages.NeedFiles,
				BuildFlags: append(goFlags.Slice(), "-toolexec="), // Explicitly disable toolexec if it's in GOFLAGS
			}

			pkgs, err := packages.Load(cfg, pattern)
			if err != nil {
				return nil, err
			}

			if len(pkgs) != 1 {
				return nil, fmt.Errorf("expected 1 package for %q, got %d", pattern, len(pkgs))
			}

			return pkgs[0], nil
		})
		if err != nil {
			return nil, fmt.Errorf("loading %q: %w", pattern, err)
		}
	}

	return resp, nil
}
