// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package buildid

import (
	"context"
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
	"sync"

	"github.com/DataDog/orchestrion/internal/fingerprint"
	"github.com/DataDog/orchestrion/internal/goflags"
	"github.com/DataDog/orchestrion/internal/injector/aspect"
	"github.com/DataDog/orchestrion/internal/injector/config"
	"github.com/DataDog/orchestrion/internal/version"
	"github.com/rs/zerolog"
	"golang.org/x/sync/errgroup"
	"golang.org/x/tools/go/packages"
)

type (
	VersionSuffixRequest  struct{}
	VersionSuffixResponse string
)

func (VersionSuffixRequest) Subject() string                  { return versionSubject }
func (VersionSuffixRequest) ResponseIs(VersionSuffixResponse) {}
func (VersionSuffixRequest) ForeachSpanTag(func(string, any)) {}

func (s *service) versionSuffix(ctx context.Context, _ VersionSuffixRequest) (VersionSuffixResponse, error) {
	log := zerolog.Ctx(ctx)

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.resolvedVersion != "" {
		s.stats.RecordHit()
		return s.resolvedVersion, nil
	}
	s.stats.RecordMiss()

	cfg, err := config.NewLoader(s.packageLoader, ".", false).Load(ctx)
	if err != nil {
		return "", fmt.Errorf("loading injector configuration: %w", err)
	}
	aspects := cfg.Aspects()

	fptr := fingerprint.New()
	defer fptr.Close()
	if err := fptr.Named("aspects", fingerprint.List[*aspect.Aspect](aspects)); err != nil {
		return "", fmt.Errorf("computing injector configuration fingerprint: %w", err)
	}

	var pkgs []*packages.Package
	if paths := aspect.InjectedPaths(aspects); len(paths) != 0 {
		flags, err := goflags.Flags(ctx)
		if err != nil {
			return "", err
		}

		pkgs, err = packages.Load(
			&packages.Config{
				Mode:       packages.NeedDeps | packages.NeedEmbedFiles | packages.NeedFiles | packages.NeedImports | packages.NeedModule,
				BuildFlags: append(flags.Except("-toolexec").Slice(), "-toolexec="), // Explicitly disable toolexec to avoid infinite recursion
				Logf:       func(format string, args ...any) { log.Trace().Str("operation", "packages.Load").Msgf(format, args...) },
			},
			paths...,
		)
		if err != nil {
			return "", err
		}
	}

	modules := make(map[string]*moduleInfo, len(pkgs))
	for _, pkg := range pkgs {
		collectModules(pkg, modules, nil)
	}
	names := make([]string, 0, len(modules))
	for name := range modules {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		mod := modules[name]
		jsonMod, err := json.Marshal(mod)
		if err != nil {
			return "", err
		}
		if err := fptr.Named(name, fingerprint.String(jsonMod)); err != nil {
			return "", err
		}
	}

	s.resolvedVersion = VersionSuffixResponse(fmt.Sprintf("orchestrion@%s%s;%s", version.Tag(), getTagSuffix(ctx), fptr.Finish()))
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
		*packages.Module
		Files [][2]string `json:"files,omitempty"`
	}{Module: m.Module}

	// If this module is replaced by a directory; we'll hash the files as well...
	if m.Replace != nil && m.Replace.Version == "" {
		toMarshal.Files = make([][2]string, 0, len(m.Files))

		var (
			pool   = sync.Pool{New: func() any { return sha512.New() }}
			errGrp errgroup.Group
			mu     sync.Mutex
		)
		for filename := range m.Files {
			errGrp.Go(func() error {
				file, err := os.Open(filename)
				if err != nil {
					return err
				}
				defer file.Close()

				sha, _ := pool.Get().(hash.Hash)
				defer func() {
					sha.Reset()
					pool.Put(sha)
				}()

				if _, err := io.Copy(sha, file); err != nil {
					return err
				}

				var buf [sha512.Size]byte
				hash := base64.URLEncoding.EncodeToString(sha.Sum(buf[:0]))

				mu.Lock()
				defer mu.Unlock()
				toMarshal.Files = append(toMarshal.Files, [2]string{filename, hash})

				return nil
			})
		}

		if err := errGrp.Wait(); err != nil {
			return nil, err
		}

		// Ensure a consistent ordering on file names...
		slices.SortFunc(toMarshal.Files, func(i, j [2]string) int {
			return strings.Compare(i[0], j[0])
		})
	}

	return json.Marshal(toMarshal)
}

var (
	tagSuffix     string
	tagSuffixOnce sync.Once
)

func getTagSuffix(ctx context.Context) string {
	tagSuffixOnce.Do(func() {
		const warningSuffix = " GOCACHE may need to be manually cleared in development iteration.\n"
		log := zerolog.Ctx(ctx)

		bi, ok := debug.ReadBuildInfo()
		if !ok {
			log.Warn().Msg("No debug.BuildInfo was found in executable." + warningSuffix)
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
			log.Warn().Err(err).Msg("Unable to read current executable." + warningSuffix)
			return
		}

		file, err := os.Open(path)
		if err != nil {
			log.Warn().Str("path", path).Err(err).Msg("Unable to open file." + warningSuffix)
			return
		}
		defer file.Close()

		sha := sha512.New()
		if _, err := io.Copy(sha, file); err != nil {
			log.Warn().Str("path", path).Err(err).Msg("Unable to hash contents of file." + warningSuffix)
			return
		}

		var data [sha512.Size]byte
		tagSuffix = "+" + base64.StdEncoding.EncodeToString(sha.Sum(data[:0]))
	})

	return tagSuffix
}
