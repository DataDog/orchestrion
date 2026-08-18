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
	"maps"
	"os"
	"slices"
	"strconv"
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
	envVarReverseVariant: func(r *ResolveRequest, path string) {
		r.reverseVariantPath = path
	},
	envVarReverseVariantFlavor: func(r *ResolveRequest, flavor string) {
		r.ReverseVariantFlavor = flavor
	},
	// Known to change between invocations & irrelevant to the resolution, but can be used to detect cycles.
	"TOOLEXEC_IMPORTPATH":       func(r *ResolveRequest, path string) { r.toolexecImportpath = path },
	envVarParentID:              func(r *ResolveRequest, id string) { r.resolveParentID = id },
	envVarResolvingTestVariants: nil,
}

type (
	ResolveRequest struct {
		Dir                string   `json:"dir"`                          // The directory to resolve from (usually where `go.mod` is)
		Env                []string `json:"env"`                          // Environment variables to use during resolution
		Pattern            string   `json:"pattern"`                      // Package pattern to resolve
		TempDir            string   `json:"tmpdir,omitempty"`             // A temporary directory to use for Go build artifacts
		TestVariantFor     string   `json:"testVariantFor,omitempty"`     // Resolve the literal Pattern as built for this package's tests
		ReverseTestVariant bool     `json:"reverseTestVariant,omitempty"` // Rebuild Pattern's test-binary import closure against the authoritative test target
		// AuthoritativeTarget is the package-under-test archive selected by the outer test-main compilation.
		AuthoritativeTarget string `json:"authoritativeTarget,omitempty"`
		// ReverseVariantFlavor preserves the stable reverse-universe identity while its temporary environment path is canonicalized out.
		ReverseVariantFlavor string `json:"reverseVariantFlavor,omitempty"`

		// Fields set by canonicalization
		resolveParentID    string // The value of the [envVarParentID] environment variable
		toolexecImportpath string // The value of the TOOLEXEC_IMPORTPATH environment variable
		reverseVariantPath string // The value of the [envVarReverseVariant] environment variable
		canonical          bool   // Whether this request was canonicalized yet
	}
	// ResolvedArchive identifies an export archive and, when non-empty, the test target for which
	// Go rebuilt it.
	ResolvedArchive struct {
		ExportFile string `json:"exportFile"`
		ForTest    string `json:"forTest,omitempty"`
	}
	// ResolveResponse maps package import paths to their resolved export archives.
	ResolveResponse map[string]ResolvedArchive
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
	if r.TestVariantFor != "" {
		set("request.test-variant-for", r.TestVariantFor)
	}
	if r.ReverseTestVariant {
		set("request.reverse-test-variant", true)
	}
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

	resolved, err := s.resolved.Load(reqHash, func() (_ resolvedPackageSet, err error) {
		if req.TestVariantFor == "" {
			return loadResolvedPackages(ctx, req, *log)
		}
		if req.ReverseTestVariant {
			return s.resolveReverseTestVariant(ctx, req, *log)
		}

		ordinaryReq := *req
		ordinaryReq.TestVariantFor = ""
		ordinaryHash, err := ordinaryReq.hash()
		if err != nil {
			return resolvedPackageSet{}, err
		}
		ordinary, err := s.resolved.Load(ordinaryHash, func() (resolvedPackageSet, error) {
			return loadResolvedPackages(ctx, &ordinaryReq, *log)
		})
		if err != nil {
			return resolvedPackageSet{}, err
		}
		if ordinary.buildFlagsErr != "" {
			return resolvedPackageSet{}, fmt.Errorf("obtaining Go build flags for test variant resolution: %s", ordinary.buildFlagsErr)
		}

		resp := maps.Clone(ordinary.response)
		config := ordinary.config
		config.Env = resolveEnvironment(ctx, req)
		config.BuildFlags = slices.Clone(config.BuildFlags)
		if ordinary.testCoverpkgInferred {
			coveragePackages := slices.Clone(ordinary.testPackagesWithoutTests)
			if !slices.Contains(coveragePackages, req.TestVariantFor) {
				coveragePackages = append(coveragePackages, req.TestVariantFor)
			}
			config.BuildFlags = scopeInferredTestCoverage(config.BuildFlags, ordinary.testCoverageMode, coveragePackages)
		}
		resp, err = mergeTestVariant(ctx, req, ordinary.packages, resp, config)
		if err != nil {
			return resolvedPackageSet{}, err
		}
		return resolvedPackageSet{response: resp}, nil
	})
	if err != nil {
		return nil, err
	}
	return resolved.response, nil
}

func loadResolvedPackages(ctx context.Context, req *ResolveRequest, log zerolog.Logger) (_ resolvedPackageSet, err error) {
	log = log.With().Str("pattern", req.Pattern).Logger()
	ctx = log.WithContext(ctx)

	span, ctx := tracer.StartSpanFromContext(ctx, "pkgs.Resolve")
	defer func() { span.Finish(tracer.WithError(err)) }()

	log.Trace().Str("dir", req.Dir).Msg("pkgs.Resolve starting")

	if req.TempDir != "" {
		if err := os.MkdirAll(req.TempDir, 0o755); err != nil {
			return resolvedPackageSet{}, fmt.Errorf("creating temporary directory %q: %w", req.TempDir, err)
		}
	}
	env := resolveEnvironment(ctx, req)

	goFlags, flagsErr := goflags.Flags(ctx)
	var flagsErrText string
	if flagsErr != nil {
		flagsErrText = flagsErr.Error()
		log.Warn().Err(flagsErr).Msg("Failed to obtain go build flags")
	}
	goFlags = goFlags.Except(
		"-a",
		"-toolexec",
	)
	testCoverpkgInferred := goFlags.TestCoverpkgInferred
	testCoverageMode, _ := goFlags.Get("-covermode")
	testPackagesWithoutTests := slices.Clone(goFlags.TestPackagesWithoutTests)
	buildFlags := append(goFlags.Slice(), fmt.Sprintf("-toolexec=%q toolexec", binpath.Orchestrion))
	if testCoverpkgInferred {
		// With implicit test coverage, packages that have tests are covered only in
		// their own test binaries. Command-line packages without tests are instead
		// covered in their ordinary form and shared by all of those binaries.
		buildFlags = scopeInferredTestCoverage(buildFlags, testCoverageMode, testPackagesWithoutTests)
	}
	loadConfig := &packages.Config{
		Context: ctx,
		Mode: packages.NeedExportFile | packages.NeedFiles |
			packages.NeedCompiledGoFiles | packages.NeedDeps | packages.NeedImports |
			packages.NeedName,
		Dir:        req.Dir,
		Env:        env,
		BuildFlags: buildFlags,
		Logf: func(format string, args ...any) {
			log.Trace().Str("operation", "packages.Load").Msgf(format, args...)
		},
	}
	pkgs, err := packages.Load(loadConfig, req.Pattern)
	if err != nil {
		log.Error().Str("pattern", req.Pattern).Err(err).Msg("pkgs.Resolve failed")
		return resolvedPackageSet{}, err
	}
	if len(pkgs) == 0 {
		return resolvedPackageSet{}, fmt.Errorf("no packages returned for pattern: %q", req.Pattern)
	}

	resp := make(ResolveResponse)
	var errs error
	for _, pkg := range pkgs {
		errs = errors.Join(errs, resp.mergeFrom(pkg))
	}
	if errs != nil {
		log.Error().Str("pattern", req.Pattern).Err(errs).Msg("pkgs.Resolve failed")
		return resolvedPackageSet{}, errs
	}

	log.Trace().Any("result", resp).Msg("pkgs.Resolve finished")
	return resolvedPackageSet{
		response:                 resp,
		packages:                 pkgs,
		config:                   *loadConfig,
		buildFlagsErr:            flagsErrText,
		testCoverpkgInferred:     testCoverpkgInferred,
		testCoverageMode:         testCoverageMode,
		testPackagesWithoutTests: testPackagesWithoutTests,
	}, nil
}

func scopeInferredTestCoverage(buildFlags []string, mode string, pkgs []string) []string {
	result := withoutCoverageBuildFlags(buildFlags)
	if len(pkgs) == 0 {
		return result
	}
	result = append(result, "-cover", "-coverpkg="+strings.Join(pkgs, ","))
	if mode != "" {
		result = append(result, "-covermode="+mode)
	}
	return result
}

func buildFlagsHaveCoverage(buildFlags []string) bool {
	enabled := false
	for _, flag := range buildFlags {
		name, value, assigned := strings.Cut(flag, "=")
		switch name {
		case "-covermode", "-coverpkg":
			enabled = true
		case "-cover":
			if !assigned {
				enabled = true
			} else if parsed, err := strconv.ParseBool(value); err == nil {
				enabled = parsed
			}
		}
	}
	return enabled
}

func withoutCoverageBuildFlags(buildFlags []string) []string {
	result := make([]string, 0, len(buildFlags)+3)
	for _, flag := range buildFlags {
		name, _, _ := strings.Cut(flag, "=")
		if name == "-cover" || name == "-covermode" || name == "-coverpkg" {
			continue
		}
		result = append(result, flag)
	}
	return result
}

func resolveEnvironment(ctx context.Context, req *ResolveRequest) []string {
	env := slices.Clone(req.Env)
	if span, ok := tracer.SpanFromContext(ctx); ok {
		tracer.Inject(span.Context(), traceutil.EnvVarCarrier{Env: &env})
	}
	if req.toolexecImportpath != "" {
		env = append(env, fmt.Sprintf("%s=%s", envVarParentID, req.toolexecImportpath))
	}
	if req.TempDir != "" {
		env = append(env, fmt.Sprintf("%s=%s", envVarGotmpdir, req.TempDir))
	}
	if req.reverseVariantPath != "" {
		env = append(env,
			envVarReverseVariant+"="+req.reverseVariantPath,
			envVarReverseVariantFlavor+"="+req.ReverseVariantFlavor,
		)
	}
	return env
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
	if pkg.PkgPath == "" || pkg.PkgPath == "unsafe" || r[pkg.PkgPath].ExportFile != "" {
		// Ignore the "unsafe" package (no archive file, ever), packages with an empty import path
		// (standard library), and those already present in the map (already processed previously).
		return nil
	}

	var errs error
	for _, err := range pkg.Errors {
		errs = errors.Join(errs, err)
	}

	r[pkg.PkgPath] = ResolvedArchive{ExportFile: pkg.ExportFile}

	for _, dep := range pkg.Imports {
		errs = errors.Join(errs, r.mergeFrom(dep))
	}

	return errs
}
