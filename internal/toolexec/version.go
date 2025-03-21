// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package toolexec

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/DataDog/orchestrion/internal/jobserver"
	"github.com/DataDog/orchestrion/internal/jobserver/buildid"
	"github.com/DataDog/orchestrion/internal/jobserver/client"
	"github.com/DataDog/orchestrion/internal/toolexec/proxy"
	"github.com/rs/zerolog"
)

// ComputeVersion returns the complete version string to be produced when the toolexec is invoked
// with `-V=full`. This invocation is used by the go toolchain to determine the complete build ID,
// ensuring the artifact cache objects are invalidated when anything in the build tooling changes.
//
// Orchestrion inserts information about itself in the string, so that we also bust cache entries if:
// - the orchestrion binary is different (instrumentation process may have changed)
// - the injector configuration is different
// - injected dependencies versions are different
func ComputeVersion(ctx context.Context, cmd proxy.Command) (string, error) {
	log := zerolog.Ctx(ctx)

	// Get the output of the raw `-V=full` invocation
	stdout := strings.Builder{}
	if err := proxy.RunCommand(ctx, cmd, func(cmd *exec.Cmd) { cmd.Stdout = &stdout }); err != nil {
		return "", err
	}

	conn, err := client.FromEnvironment(ctx, "")
	if err != nil {
		if !errors.Is(err, client.ErrNoServerAvailable) {
			return "", err
		}
		log.Debug().Msg("No job server available; starting an in-process temporary server...")
		server, err := jobserver.New(ctx, &jobserver.Options{NoListener: true})
		if err != nil {
			return "", err
		}
		defer server.Shutdown()
		if conn, err = server.Connect(); err != nil {
			return "", err
		}
	}

	res, err := client.Request(ctx, conn, buildid.VersionSuffixRequest{})
	if err != nil {
		return "", err
	}

	// Produce the complete version string
	return fmt.Sprintf("%s:%s", strings.TrimSpace(stdout.String()), res), nil
}
