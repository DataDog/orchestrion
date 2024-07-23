// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package pkgs

import (
	"bytes"
	"crypto/sha512"
	"encoding/base64"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/datadog/orchestrion/internal/jobserver/client"
	"github.com/datadog/orchestrion/internal/jobserver/common"
	"github.com/nats-io/nats.go"
	"golang.org/x/tools/go/packages"
)

type (
	ResolveRequest struct {
		Dir        string   `json:"dir"`                  // The directory to resolve from (usually where `go.mod` is)
		Env        []string `json:"env"`                  // Environment variables to use during resolution
		BuildFlags []string `json:"buildFlags,omitempty"` // Additional build flags to pass to the resolution driver
		Patterns   []string `json:"patterns"`             // Package patterns to resolve

		canonical bool // Whether this request was canonicalized yet
	}
	// ResolveResponse is a map of package import path to their respective export file, if one is
	// found. Users should handle possibly missing export files as is relevant to their use-case.
	ResolveResponse map[string]string
)

func (*ResolveRequest) Subject() string {
	return resolveSubject
}

func (ResolveResponse) IsResponseTo(*ResolveRequest) {}

func (s *service) resolve(msg *nats.Msg) {
	var req ResolveRequest
	dec := gob.NewDecoder(bytes.NewReader(msg.Data))
	if err := dec.Decode(&req); err != nil {
		common.Respond[ResolveResponse](msg, nil, err)
		return
	}
	// Make sure all children jobs connect to THIS jobserver; this is more efficient than checking for
	// the local file system beacon.
	req.Env = append(req.Env, fmt.Sprintf("%s=%s", client.ENV_VAR_JOBSERVER_URL, s.serverURL))
	req.canonicalize()

	reqHash, err := req.hash()
	if err != nil {
		common.Respond[ResolveResponse](msg, nil, err)
		return
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
				Env:        req.Env,
				BuildFlags: req.BuildFlags,
			},
			req.Patterns...,
		)
		if err != nil {
			return nil, err
		}

		var resp ResolveResponse
		for _, pkg := range pkgs {
			resp.mergeFrom(pkg)
		}
		return resp, nil
	})
	if err != nil {
		common.Respond[ResolveResponse](msg, nil, err)
		return
	}

	common.Respond(msg, resp, nil)
}

func hashArray(items []string) string {
	h := sha512.New512_224()

	for idx, item := range items {
		fmt.Fprintf(h, "\x01%d\x02%s\x03", idx, item)
	}

	return base64.URLEncoding.EncodeToString(h.Sum(nil))
}

func (r *ResolveRequest) canonicalize() {
	if r.canonical {
		return
	}

	slices.Sort(r.BuildFlags)
	slices.Sort(r.Patterns)
	r.Env = canonicalizeEnviron(r.Env)

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

func (r *ResolveResponse) mergeFrom(pkg *packages.Package) {
	if pkg.PkgPath == "" || pkg.PkgPath == "unsafe" {
		return
	}
	if *r == nil {
		*r = make(ResolveResponse)
	}
	(*r)[pkg.PkgPath] = pkg.ExportFile

	for _, dep := range pkg.Imports {
		r.mergeFrom(dep)
	}
}

var envIgnoreList = map[string]struct{}{
	"PWD":                 {}, // We don't use this, instead rely on the `Dir` field.
	"TOOLEXEC_IMPORTPATH": {}, // Known to change between invocations & irrelevant to the resolution.
}

func canonicalizeEnviron(env []string) []string {
	named := make(map[string]string, len(env))
	names := make([]string, 0, len(env))

	for _, kv := range env {
		name, _, _ := strings.Cut(kv, "=")
		if _, ignore := envIgnoreList[name]; ignore {
			continue
		}
		if _, found := named[name]; !found {
			names = append(names, name)
		}
		named[name] = kv
	}

	slices.Sort(names)
	result := make([]string, 0, len(names))
	for _, name := range names {
		result = append(result, named[name])
	}
	return result
}
