// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package pkgs

import (
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/DataDog/orchestrion/internal/jobserver/client"
	"github.com/DataDog/orchestrion/internal/log"
	"golang.org/x/tools/go/packages"
)

const (
	envVarParentID = "ORCHESTRION_PKG.RESOLVE_PARENT_ID"
)

var envIgnoreList = map[string]func(*ResolveRequest, string){
	// We don't use this, instead rely on the [ResolveRequest.Dir] field.
	"PWD": nil,
	// Known to change between invocations & irrelevant to the resolution, but can be used to detect cycles.
	"TOOLEXEC_IMPORTPATH": func(r *ResolveRequest, path string) { r.toolexecImportpath = path },
	envVarParentID:        func(r *ResolveRequest, id string) { r.resolveParentID = id },
}

type (
	ResolveRequest struct {
		Dir        string   `json:"dir"`                  // The directory to resolve from (usually where `go.mod` is)
		Env        []string `json:"env"`                  // Environment variables to use during resolution
		BuildFlags []string `json:"buildFlags,omitempty"` // Additional build flags to pass to the resolution driver
		Pattern    string   `json:"pattern"`              // Package pattern to resolve

		// Fields set by canonicalization
		resolveParentID    string // The value of the [envVarParentID] environment variable
		toolexecImportpath string // The value of the TOOLEXEC_IMPORTPATH environment variable
		canonical          bool   // Whether this request was canonicalized yet
	}
	// ResolveResponse is a map of package import path to their respective export file, if one is
	// found. Users should handle possibly missing export files as is relevant to their use-case.
	ResolveResponse map[string]string
)

func NewResolveRequest(dir string, buildFlags []string, pattern string) *ResolveRequest {
	// We add the `-toolexec` flags here (client-side) because it otherwise makes it difficult to test
	// the implementation of the resolver without causing the tests to recursively spawn temselves.
	allFlags := make([]string, 0, len(buildFlags)+1)
	allFlags = append(allFlags, buildFlags...)
	allFlags = append(allFlags, fmt.Sprintf("-toolexec=%q toolexec", os.Args[0]))

	return &ResolveRequest{
		Dir:        dir,
		Env:        os.Environ(),
		BuildFlags: allFlags,
		Pattern:    pattern,
	}
}

func (*ResolveRequest) Subject() string {
	return resolveSubject
}

func (ResolveResponse) IsResponseTo(*ResolveRequest) {}

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

func (s *service) resolve(req *ResolveRequest) (ResolveResponse, error) {
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

	env := req.Env
	if req.toolexecImportpath != "" {
		env = make([]string, 0, len(req.Env)+1)
		env = append(env, req.Env...)
		env = append(env, fmt.Sprintf("%s=%s", envVarParentID, req.toolexecImportpath))
	}
	resp, err := s.resolved.Load(reqHash, func() (ResolveResponse, error) {
		pkgs, err := packages.Load(
			&packages.Config{
				Mode:
				// We need the export file (the whole point of the resolution)
				packages.NeedExportFile |
					// We want to also resolve transitive dependencies, so we need Deps & Imports
					packages.NeedDeps | packages.NeedImports |
					// Finally, we need the resolved package import path
					packages.NeedName,
				Dir:        req.Dir,
				Env:        env,
				BuildFlags: req.BuildFlags,
				Logf:       func(format string, args ...any) { log.Infof("[JOBSERVER] packages.Load -- "+format+"\n", args...) },
			},
			req.Pattern,
		)
		if err != nil {
			return nil, err
		}

		resp := make(ResolveResponse)
		for _, pkg := range pkgs {
			resp.mergeFrom(pkg)
		}
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

	slices.Sort(r.BuildFlags)
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

func (r ResolveResponse) mergeFrom(pkg *packages.Package) {
	if pkg.PkgPath == "" || pkg.PkgPath == "unsafe" || r[pkg.PkgPath] != "" {
		// Ignore the "unsafe" package (no archive file, ever), packages with an empty import path
		// (standard library), and those already present in the map (already processed previously).
		return
	}

	for _, err := range pkg.Errors {
		log.Errorf("[JOBSERVER] Error during resolution of %q: %v\n", pkg.PkgPath, err)
	}

	r[pkg.PkgPath] = pkg.ExportFile
	for _, dep := range pkg.Imports {
		r.mergeFrom(dep)
	}
}
