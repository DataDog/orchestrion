// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package buildid

import (
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"hash"
	"io"
	"os"
	"runtime/debug"
	"slices"
	"sort"
	"strings"

	"github.com/datadog/orchestrion/internal/goflags"
	"github.com/datadog/orchestrion/internal/injector/builtin"
	"github.com/datadog/orchestrion/internal/log"
	"github.com/datadog/orchestrion/internal/version"
	"golang.org/x/tools/go/packages"
)

type (
	VersionSuffixRequest  struct{}
	VersionSuffixResponse string
)

func (*VersionSuffixRequest) Subject() string {
	return versionSubject
}

func (VersionSuffixResponse) IsResponseTo(*VersionSuffixRequest) {}

var tagSuffix string

func (s *service) versionSuffix(req *VersionSuffixRequest) (VersionSuffixResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.resolvedVersion != "" {
		s.stats.RecordHit()
		return s.resolvedVersion, nil
	}
	s.stats.RecordMiss()

	cwd, _ := os.Getwd()
	log.Tracef("[JOBSERVER/%s] Starting resolution of the version suffix (PWD=%q)...\n", versionSubject, cwd)

	// We need to forward go flags that are relevant to this build... For example, if this is a coverage-enabled build,
	// we need to ensure the build ID reflects that (injected packages need to be coverage-enabled, too).
	var buildFlags []string
	if flags, err := goflags.Flags(); err != nil {
		log.Errorf("Failed to retrieve go command flags: %v\n", err)
	} else {
		buildFlags = flags.Slice()
	}
	// Explicitly disable toolexec to avoid infinite recursion
	buildFlags = append(buildFlags, "-toolexec=")

	log.Tracef("[JOBSERVER/%s] Loading dependencies with build flags: %q\n", versionSubject, buildFlags)
	pkgs, err := packages.Load(
		&packages.Config{
			Mode:       packages.NeedDeps | packages.NeedEmbedFiles | packages.NeedFiles | packages.NeedImports | packages.NeedModule,
			BuildFlags: buildFlags,
		},
		builtin.InjectedPaths[:]...,
	)
	if err != nil {
		return "", err
	}

	modules := make(map[string]*moduleInfo)
	for _, pkg := range pkgs {
		collectModules(pkg, modules, nil)
	}
	names := make([]string, 0, len(modules))
	for name := range modules {
		names = append(names, name)
	}
	sort.Strings(names)

	sha := sha512.New()
	for _, name := range names {
		mod := modules[name]
		if _, err := fmt.Fprintf(sha, "\x01%s\x02", name); err != nil {
			return "", err
		}

		if data, err := json.Marshal(mod); err != nil {
			return "", err
		} else if _, err := sha.Write(data); err != nil {
			return "", err
		}
	}
	var data [sha512.Size]byte
	sum := base64.StdEncoding.EncodeToString(sha.Sum(data[:0]))

	s.resolvedVersion = VersionSuffixResponse(fmt.Sprintf(
		"orchestrion@%s%s;injectables=%s;rules=%s",
		version.Tag, tagSuffix,
		sum,
		builtin.Checksum,
	))

	log.Tracef("[JOBSERVER/%s] Resolved version suffix: %s\n", versionSubject, s.resolvedVersion)
	return s.resolvedVersion, nil
}

type moduleInfo struct {
	*packages.Module
	Files map[string]struct{}
}

func collectModules(pkg *packages.Package, modules map[string]*moduleInfo, knownIDs map[string]struct{}) {
	if _, known := knownIDs[pkg.ID]; known {
		return
	} else if knownIDs == nil {
		knownIDs = make(map[string]struct{})
	}
	knownIDs[pkg.ID] = struct{}{}

	if pkg.Module != nil {
		info := modules[pkg.Module.Path]
		if info == nil {
			info = &moduleInfo{Module: pkg.Module, Files: make(map[string]struct{})}
			modules[pkg.Module.Path] = info
		}

		for _, files := range [...][]string{pkg.GoFiles, pkg.EmbedFiles, pkg.OtherFiles} {
			for _, file := range files {
				info.Files[file] = struct{}{}
			}
		}
	}
	for _, imp := range pkg.Imports {
		if imp.Module == nil {
			continue
		}
		collectModules(imp, modules, knownIDs)
	}
}

var _ json.Marshaler = (*moduleInfo)(nil)

func (m *moduleInfo) MarshalJSON() ([]byte, error) {
	toMarshal := struct {
		*packages.Module `json:",inline"`
		Files            [][2]string `json:"files,omitempty"`
	}{Module: m.Module}

	// If this module is replaced by a directory; we'll hash the files as well...
	if m.Replace != nil && m.Replace.Version == "" {
		var (
			sha hash.Hash
			sum [sha512.Size]byte
		)

		toMarshal.Files = make([][2]string, 0, len(m.Files))
		for file := range m.Files {
			if sha == nil {
				sha = sha512.New()
			} else {
				sha.Reset()
			}

			if err := func() error {
				file, err := os.Open(file)
				if err != nil {
					return err
				}
				defer file.Close()
				if _, err := io.Copy(sha, file); err != nil {
					return err
				}
				return nil
			}(); err != nil {
				return nil, err
			}

			hash := base64.StdEncoding.EncodeToString(sha.Sum(sum[:0]))
			toMarshal.Files = append(toMarshal.Files, [2]string{file, hash})
		}
		// Ensure a consistent ordering on file names...
		slices.SortFunc(toMarshal.Files, func(i, j [2]string) int {
			return strings.Compare(i[0], j[0])
		})
	}

	return json.Marshal(toMarshal)
}

func init() {
	const warningSuffix = " GOCACHE may need to be manually cleared in development iteration.\n"

	bi, ok := debug.ReadBuildInfo()
	if !ok {
		log.Warnf("No debug.BuildInfo was found in executable." + warningSuffix)
		return
	}

	// If the version is "(devel)", the command was built from a development
	// tree. It is typically empty when running test suites (via `test_main`).
	isDev := bi.Main.Version == "(devel)" || bi.Main.Version == ""

	if !isDev {
		// The build has a version... but was it from a clean tree? If not, it still
		// is a dev build!
		for _, setting := range bi.Settings {
			if setting.Key == "vcs.modified" {
				isDev = setting.Value == "true"
				break
			}
		}
	}

	if !isDev {
		// At this stage we don't think this is a dev build, so we don't need a
		// tag suffix.
		return
	}

	// We're in a dev build, so we'll add a checksum of this executable as the tag
	// suffix, so that development iteration isn't frustrated by needing to clear
	// the GOCACHE over and over again.
	path, err := os.Executable()
	if err != nil {
		log.Warnf("Unable to read current executable: %v."+warningSuffix, err)
		return
	}

	file, err := os.Open(path)
	if err != nil {
		log.Warnf("Unable to open %q: %v."+warningSuffix, path, err)
		return
	}
	defer file.Close()

	sha := sha512.New()
	if _, err := io.Copy(sha, file); err != nil {
		log.Warnf("Unable to hash contents of %q: %v."+warningSuffix, path, err)
		return
	}

	var data [sha512.Size]byte
	tagSuffix = "+" + base64.StdEncoding.EncodeToString(sha.Sum(data[:0]))
}
