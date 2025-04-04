// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package pkgs

import (
	"context"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/DataDog/dd-trace-go/v2/ddtrace/tracer"
	"github.com/DataDog/orchestrion/internal/binpath"
	"github.com/DataDog/orchestrion/internal/goflags"
	"github.com/DataDog/orchestrion/internal/jobserver/client"
	"github.com/DataDog/orchestrion/internal/traceutil"
	"github.com/rs/zerolog"
	"golang.org/x/tools/go/packages"
)

const (
	envVarParentID = "ORCHESTRION_PKG.RESOLVE_PARENT_ID"
	envVarGotmpdir = "GOTMPDIR"
)

var envIgnoreList = map[string]func(*ResolveRequest, string){
	// We don't use this, instead rely on the [ResolveRequest.Dir] field.
	"PWD": nil,
	// We override `GOTMPDIR` with the [ResolveRequest.TempDir] field.
	envVarGotmpdir: func(r *ResolveRequest, dir string) {
		if r.TempDir != "" {
			return
		}
		r.TempDir = dir
	},
	// Known to change between invocations & irrelevant to the resolution, but can be used to detect cycles.
	"TOOLEXEC_IMPORTPATH": func(r *ResolveRequest, path string) { r.toolexecImportpath = path },
	envVarParentID:        func(r *ResolveRequest, id string) { r.resolveParentID = id },
}

type (
	ResolveRequest struct {
		Dir     string   `json:"dir"`              // The directory to resolve from (usually where `go.mod` is)
		Env     []string `json:"env"`              // Environment variables to use during resolution
		Pattern string   `json:"pattern"`          // Package pattern to resolve
		TempDir string   `json:"tmpdir,omitempty"` // A temporary directory to use for Go build artifacts

		// Fields set by canonicalization
		resolveParentID    string // The value of the [envVarParentID] environment variable
		toolexecImportpath string // The value of the TOOLEXEC_IMPORTPATH environment variable
		canonical          bool   // Whether this request was canonicalized yet
	}
	// ResolveResponse is a map of package import path to their respective export file, if one is
	// found. Users should handle possibly missing export files as is relevant to their use-case.
	ResolveResponse map[string]string
)

func NewResolveRequest(dir string, pattern string) ResolveRequest {
	return ResolveRequest{
		Dir:     dir,
		Env:     os.Environ(),
		Pattern: pattern,
	}
}

func (ResolveRequest) Subject() string            { return resolveSubject }
func (ResolveRequest) ResponseIs(ResolveResponse) {}
func (r ResolveRequest) ForeachSpanTag(set func(key string, value any)) {
	set("request.dir", r.Dir)
	set("request.pattern", r.Pattern)
}

func (r *ResolveRequest) canonicalizeEnviron() {
	named := make(map[string]string, len(r.Env))
	names := make([]string, 0, len(r.Env))

	for _, kv := range r.Env {
		name, val, _ := strings.Cut(kv, "=")
		if cb, ignore := envIgnoreList[name]; ignore {
			if cb != nil {
				cb(r, val)
			}
			continue
		}
		if _, found := named[name]; !found {
			names = append(names, name)
		}
		named[name] = kv
	}

	slices.Sort(names)
	r.Env = make([]string, 0, len(names))
	for _, name := range names {
		r.Env = append(r.Env, named[name])
	}
}

func (s *service) resolve(ctx context.Context, req *ResolveRequest) (ResolveResponse, error) {
	log := zerolog.Ctx(ctx)

	// Make sure all children jobs connect to THIS jobserver; this is more efficient than checking for
	// the local file system beacon.
	req.Env = append(req.Env, fmt.Sprintf("%s=%s", client.EnvVarJobserverURL, s.serverURL))
	req.canonicalize()

	reqHash, err := req.hash()
	if err != nil {
		return nil, err
	}

	if req.resolveParentID != "" {
		if err := s.graph.AddEdge(req.resolveParentID, req.toolexecImportpath); err != nil {
			return nil, err
		}
		defer s.graph.RemoveEdge(req.resolveParentID, req.toolexecImportpath)
	}

	resp, err := s.resolved.Load(reqHash, func() (_ ResolveResponse, err error) {
		log := log.With().Str("pattern", req.Pattern).Logger()
		ctx := log.WithContext(ctx)

		span, ctx := tracer.StartSpanFromContext(ctx, "pkgs.Resolve")
		defer func() { span.Finish(tracer.WithError(err)) }()

		log.Trace().Str("dir", req.Dir).Msg("pkgs.Resolve starting")

		env := slices.Clone(req.Env)
		tracer.Inject(span.Context(), traceutil.EnvVarCarrier{Env: &env})
		if req.toolexecImportpath != "" {
			env = make([]string, 0, len(req.Env)+1)
			env = append(env, req.Env...)
			env = append(env, fmt.Sprintf("%s=%s", envVarParentID, req.toolexecImportpath))
		}
		if req.TempDir != "" {
			// Make sure the directory exists (go blindly assumes that...)
			if err := os.MkdirAll(req.TempDir, 0o755); err != nil {
				return nil, fmt.Errorf("creating temporary directory %q: %w", req.TempDir, err)
			}
			env = append(env, fmt.Sprintf("%s=%s", envVarGotmpdir, req.TempDir))
		}

		goFlags, err := goflags.Flags(ctx)
		if err != nil {
			log.Warn().Err(err).Msg("Failed to obtain go build flags")
		}
		goFlags = goFlags.Except(
			"-a",        // Re-building everything here would be VERY expensive, as we'd re-build a lot of stuff multiple times
			"-toolexec", // We'll override `-toolexec` later with `orchestrion toolexec`, no need to pass multiple times...
		)

		buildFlags := append(
			goFlags.Slice(),
			fmt.Sprintf("-toolexec=%q toolexec", binpath.Orchestrion),
		)

		pkgs, err := packages.Load(
			&packages.Config{
				Mode:
				// We need the export file (the whole point of the resolution)
				packages.NeedExportFile |
					// We want to also resolve transitive dependencies, so we need Deps & Imports. We also
					// need CompiledGoFiles in order to see imports possibly added by the toolchain (cgo,
					// cover, etc...)
					packages.NeedCompiledGoFiles | packages.NeedDeps | packages.NeedImports |
					// Finally, we need the resolved package import path
					packages.NeedName,
				Dir:        req.Dir,
				Env:        env,
				BuildFlags: buildFlags,
				Logf:       func(format string, args ...any) { log.Trace().Str("operation", "packages.Load").Msgf(format, args...) },
			},
			req.Pattern,
		)
		if err != nil {
			log.Error().Str("pattern", req.Pattern).Err(err).Msg("pkgs.Resolve failed")
			return nil, err
		}
		if len(pkgs) == 0 {
			return nil, fmt.Errorf("no packages returned for pattern: %q", req.Pattern)
		}

		resp := make(ResolveResponse)
		var errs error
		for _, pkg := range pkgs {
			errs = errors.Join(errs, resp.mergeFrom(pkg))
		}

		if errs != nil {
			log.Error().Str("pattern", req.Pattern).Err(errs).Msg("pkgs.Resolve failed")
			return nil, errs
		}

		log.Trace().Any("result", resp).Msg("pkgs.Resolve finished")
		return resp, nil
	})
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (r *ResolveRequest) canonicalize() {
	if r.canonical {
		return
	}
	r.canonicalizeEnviron()
	r.canonical = true
}

func (r *ResolveRequest) hash() (string, error) {
	hash := sha512.New()
	encoder := json.NewEncoder(hash)

	r.canonicalize()
	if err := encoder.Encode(r); err != nil {
		return "", err
	}

	var sum [sha512.Size]byte
	return base64.URLEncoding.EncodeToString(hash.Sum(sum[:0])), nil
}

func (r ResolveResponse) mergeFrom(pkg *packages.Package) error {
	if pkg.PkgPath == "" || pkg.PkgPath == "unsafe" || r[pkg.PkgPath] != "" {
		// Ignore the "unsafe" package (no archive file, ever), packages with an empty import path
		// (standard library), and those already present in the map (already processed previously).
		return nil
	}

	var errs error
	for _, err := range pkg.Errors {
		errs = errors.Join(errs, err)
	}

	r[pkg.PkgPath] = pkg.ExportFile

	for _, dep := range pkg.Imports {
		errs = errors.Join(errs, r.mergeFrom(dep))
	}

	return errs
}
