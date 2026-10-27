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
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/DataDog/dd-trace-go/v2/ddtrace/tracer"
	"github.com/DataDog/orchestrion/internal/traceutil"
)

const (
	envVarParentID = "ORCHESTRION_PKG.RESOLVE_PARENT_ID"
	envVarGotmpdir = "GOTMPDIR"
)

// ResolveParentImportPath returns the import path of the package whose
// compilation triggered the resolution that spawned the current process' build,
// if any. It is blank in builds the user started directly.
func ResolveParentImportPath() string {
	return os.Getenv(envVarParentID)
}

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

type ResolveRequest struct {
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
