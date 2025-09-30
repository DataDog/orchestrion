// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package ensure

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/DataDog/orchestrion/internal/goenv"
	"github.com/DataDog/orchestrion/internal/gomod"
	"github.com/DataDog/orchestrion/internal/integrations"
	"github.com/rs/zerolog"
	"golang.org/x/mod/semver"
)

func RequiredIntegrations(ctx context.Context, goMod string) ([]gomod.Edit, error) {
	log := zerolog.Ctx(ctx)

	curMod, err := gomod.Parse(ctx, goMod)
	if err != nil {
		return nil, fmt.Errorf("parsing %q: %w", goMod, err)
	}

	// V1
	if ver, found := curMod.Requires(integrations.DatadogTracerV1); found && semver.Compare(ver, "v1.74.0") < 0 {
		if err := gomod.RunGet(ctx, goMod, integrations.DatadogTracerV1+"@latest"); err != nil {
			return nil, fmt.Errorf("go get "+integrations.DatadogTracerV1+"@latest: %w", err)
		}
	}

	// V2
	shouldUpgrade, targetVersion, err := resolveIntegrationVersion(ctx, curMod, integrations.DatadogTracerV2)
	if err != nil {
		return nil, fmt.Errorf("resolving %s version: %w", integrations.DatadogTracerV2, err)
	}
	if shouldUpgrade {
		log.Info().
			Str("target", targetVersion).
			Msg(fmt.Sprintf("Installing or upgrading %s (via %s)", integrations.DatadogTracerV2, integrations.DatadogTracerV2All))
		// We install/upgrade the `orchestrion/all/v2` module as it includes all interesting contribs in its dependency
		// closure, so we don't have to manually verify all of them. The `go mod tidy` later will clean up if needed.
		target := integrations.DatadogTracerV2All + "@" + targetVersion
		if err := gomod.RunGet(ctx, goMod, target); err != nil {
			return nil, fmt.Errorf("go get "+target+": %w", err)
		}
	}

	var edits []gomod.Edit
	_, found := curMod.Requires(integrations.DatadogTracerV2All)
	if found {
		edits = append(edits, gomod.Require{Path: integrations.DatadogTracerV2All, Version: targetVersion})
	}
	return edits, nil
}

// versionFetcher is a function type for fetching latest versions
type versionFetcher func(ctx context.Context, modPath string) (string, error)

// resolveIntegrationVersion determines if the specified integration should be upgraded, and to which version.
func resolveIntegrationVersion(ctx context.Context, curMod gomod.File, integration string) (bool, string, error) {
	return resolveIntegrationVersionWithFetcher(ctx, curMod, integration, fetchLatestVersion)
}

// resolveIntegrationVersionWithFetcher is the internal implementation that accepts a custom version fetcher.
// This allows for easier testing by injecting a mock fetcher.
func resolveIntegrationVersionWithFetcher(ctx context.Context, curMod gomod.File, integration string, fetcher versionFetcher) (bool, string, error) {
	log := zerolog.Ctx(ctx)

	foundVersion, found := curMod.Requires(integration)
	shippedVersion := fetchShippedVersions()[integration]
	latestVersion, err := fetcher(ctx, integration)
	if err != nil {
		return false, "", fmt.Errorf("fetching latest version for %s: %w", integration, err)
	}
	log.Debug().
		Str("module", integrations.DatadogTracerV2).
		Str("current", foundVersion).
		Str("shipped", shippedVersion).
		Str("latest", latestVersion).
		Msg("Checking for updates")

	var (
		shouldUpgrade bool
		targetVersion string
	)

	// Only run go get if we need to upgrade or if the module is not present.
	if found {
		targetVersion = foundVersion
		if semver.Compare(foundVersion, "v2.1.0") < 0 {
			shouldUpgrade = true
			targetVersion = maxVersion(shippedVersion, latestVersion)
		} else {
			// Force uograde, otherwise, go mod tidy will fail.
			if semver.Compare(shippedVersion, foundVersion) > 0 {
				shouldUpgrade = true
				targetVersion = shippedVersion
			}
		}
	} else {
		shouldUpgrade = true
		targetVersion = maxVersion(shippedVersion, latestVersion)
	}
	return shouldUpgrade, targetVersion, nil
}

// fetchLatestVersion queries the Go module registry to get the actual latest version
// of the specified module path.
func fetchLatestVersion(ctx context.Context, modPath string) (string, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "go", "list", "-m", "-json", modPath+"@latest")

	// Build environment with GOTOOLCHAIN=local and explicit -mod=mod
	// This is necessary because GOFLAGS might contain -mod=vendor which prevents
	// querying the module registry for @latest versions.
	env := os.Environ()
	env = append(env, "GOTOOLCHAIN=local")

	// Remove any existing GOFLAGS that might contain -mod=vendor
	var cleanEnv []string
	for _, e := range env {
		if !strings.HasPrefix(e, "GOFLAGS=") {
			cleanEnv = append(cleanEnv, e)
		}
	}
	// Set GOFLAGS with -mod=mod to ensure we can query the registry
	cleanEnv = append(cleanEnv, "GOFLAGS=-mod=mod")

	cmd.Env = cleanEnv
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("go list -m -json %s@latest: %w (stderr: %s)", modPath, err, stderr.String())
	}

	var modInfo struct {
		Version string `json:"Version"`
	}
	if err := json.NewDecoder(&stdout).Decode(&modInfo); err != nil {
		return "", fmt.Errorf("decoding go list output: %w", err)
	}

	return modInfo.Version, nil
}

func maxVersion(versions ...string) string {
	semver.Sort(versions)
	return versions[len(versions)-1]
}

func resolveDependencyVersion(modDir string, dependency string) (string, error) {
	goMod, err := goenv.GOMOD(modDir)
	if err != nil {
		return "", fmt.Errorf("getting GOMOD: %w", err)
	}
	mod, err := gomod.Parse(context.Background(), goMod)
	if err != nil {
		return "", fmt.Errorf("parsing %q: %w", goMod, err)
	}
	ver, ok := mod.Requires(dependency)
	if !ok {
		return "", fmt.Errorf("failed to find %q in %q", dependency, goMod)
	}
	return ver, nil
}

var (
	initOnce                   sync.Once
	orchestrionShippedVersions = atomic.Pointer[map[string]string]{}
)

func fetchShippedVersions() map[string]string {
	initOnce.Do(func() {
		versions := make(map[string]string)
		_, thisFile, _, _ := runtime.Caller(0)
		// The version of dd-trace-go that shipped with the current version of orchestrion.
		// We use this to determine if we need to upgrade dd-trace-go when pinning.
		orchestrionRoot := filepath.Join(thisFile, "..", "..", "..")
		ver, err := resolveDependencyVersion(orchestrionRoot, integrations.DatadogTracerV2)
		if err != nil {
			panic(fmt.Errorf("resolving %s version in %q: %w", integrations.DatadogTracerV2, orchestrionRoot, err))
		}
		versions[integrations.DatadogTracerV2] = ver
		instrumentationRoot := filepath.Join(orchestrionRoot, "instrument")
		ver, err = resolveDependencyVersion(instrumentationRoot, integrations.DatadogTracerV2All)
		if err != nil {
			panic(fmt.Errorf("resolving %s version in %q: %w", integrations.DatadogTracerV2All, instrumentationRoot, err))
		}
		versions[integrations.DatadogTracerV2All] = ver
		orchestrionShippedVersions.Store(&versions)
	})
	return *orchestrionShippedVersions.Load()
}
