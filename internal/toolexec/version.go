// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package toolexec

import (
	"bytes"
	"crypto/sha512"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strings"

	"github.com/datadog/orchestrion/internal/golist"
	"github.com/datadog/orchestrion/internal/injector/builtin"
	"github.com/datadog/orchestrion/internal/log"
	"github.com/datadog/orchestrion/internal/toolexec/proxy"
	"github.com/datadog/orchestrion/internal/version"
)

// ComputeVersion returns the complete version string to be produced when the toolexec is invoked
// with `-V=full`. This invocation is used by the go toolchain to determine the complete build ID,
// ensuring the artifact cache objects are invalidated when anything in the build tooling changes.
//
// Orchestrion inserts information about itself in the string, so that we also bust cache entries if:
// - the orchestrion binary is different (instrumentation process may have changed)
// - the injector configuration is different
// - injected dependencies versions are different
func ComputeVersion(cmd proxy.Command, orchestrionBinPath string) string {
	// Get the output of the raw `-V=full` invocation
	stdout := strings.Builder{}
	proxy.MustRunCommand(cmd, func(cmd *exec.Cmd) { cmd.Stdout = &stdout })

	// Check if this is a development build, and if so add a checksum of the binary to the version
	// string so different dev builds use different change entries (impairing the peak performance of
	// builds with those, but removing the risk of object cache producing false positives/negatives in
	// test runs).
	versionString := bytes.NewBufferString(version.Tag)
	if bi, ok := debug.ReadBuildInfo(); ok {
		var vcsModified bool
		for _, setting := range bi.Settings {
			if setting.Key == "vcs.modified" {
				vcsModified = setting.Value == "true"
				break
			}
		}

		if vcsModified || bi.Main.Version == "(devel)" {
			// If this binary was built with `go build`, it may have VCS information indicating the
			// working directory was dirty (vcsModified). If it was produced with `go run`, it won't
			// have VCS information, but the version may be `(devel)`, indicating it was built from a
			// development branch. In either case, we add a checksum of the current binary to the
			// version string so that development iteration builds aren't frustrated by GOCACHE.
			// We would have wanted to use `bi.Main.Sum` and `bi.Deps.*.Sum` here instead, but the go
			// toolchain does not produce `bi.Main.Sum`, which prevents detecting changes in the main
			// module itself.
			log.Tracef("Detected this build is from a dev tree: vcs.modified=%v; main.Version=%s\n", vcsModified, bi.Main.Version)

			// We try to open the executable. If that fails, we won't be able to hash it, but we'll
			// ignore this error. The consequence is that GOCACHE entries may be re-used when they
			// shouldn't; which is only a problem on dev iteration. On Windows specifically, this may
			// always fail due to being unable to open a running executable for reading.
			if file, err := os.Open(orchestrionBinPath); err == nil {
				sha := sha512.New512_224()
				var buffer [4_096]byte
				if _, err := io.CopyBuffer(sha, file, buffer[:]); err == nil {
					var buf [sha512.Size224]byte
					fmt.Fprintf(versionString, "+%02x", sha.Sum(buf[:0]))
				} else {
					log.Debugf("When hashing executable file: %v\n", err)
				}
			} else {
				// This can happen, e.g, on Windows depending on  the file system.
				log.Debugf("When opening executable file for hashing: %v\n", err)
			}
		}
	}

	// Simply hash the output of `go list -deps -json` to detect changes in possibly injected
	// dependencies. We pass `-toolexec` in order to NOT honor it from GOFLAGS, as it would cause an
	// infinite recursive invocation of Orchestrion (obviously not desirable).
	goList := exec.Command("go", "list", "-deps", "-json", "-toolexec=", "--")
	goList.Args = append(goList.Args, builtin.InjectedPaths[:]...)
	var jsonText bytes.Buffer
	goList.Stdout = &jsonText
	goList.Stderr = os.Stderr
	if err := goList.Run(); err != nil {
		panic(fmt.Errorf("failed to run go list ...: %w", err))
	}
	depsHash := sha512.New()
	if _, err := fmt.Fprint(depsHash, jsonText.String()); err != nil {
		panic(fmt.Errorf("while hashing dependencies list: %w", err))
	}

	// For any directory-targeting replace directive, also hash the source files...
	parsed, err := golist.ParseJSON(&jsonText)
	if err != nil {
		panic(fmt.Errorf("parsing output og 'go list -json ...': %w", err))
	}
	for _, entry := range parsed {
		if entry.Standard || entry.Module == nil || entry.Module.Replace == nil || entry.Module.Replace.Version != "" {
			continue
		}
		for _, file := range entry.AllFiles(false) {
			filepath := filepath.Join(entry.Dir, file)
			fmt.Fprintf(depsHash, "\x01%s\x02", filepath)
			file, err := os.Open(filepath)
			if err != nil {
				panic(fmt.Errorf("opening %q: %w", filepath, err))
			}
			defer file.Close()
			if _, err := io.Copy(depsHash, file); err != nil {
				panic(fmt.Errorf("hashing %q: %w", filepath, err))
			}
			fmt.Fprint(depsHash, "\x03")
		}
	}

	var depsChecksum [sha512.Size]byte
	depsHashString := base64.StdEncoding.EncodeToString(depsHash.Sum(depsChecksum[:0]))

	// Produce the complete version string
	return fmt.Sprintf("%s:%s,%s,%s",
		// Original version string (produced by the go tool)
		strings.TrimSpace(stdout.String()),
		// Orchestrion's version + adornments
		versionString.String(),
		// Checksum of the built-in rule set
		builtin.Checksum,
		// Hash of (potentially) injected module version information
		depsHashString,
	)
}
