// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package pkgs

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResolveEnvironmentUsesCurrentRequester(t *testing.T) {
	req := ResolveRequest{Env: []string{
		"A=value",
		envVarParentID + "=stale-requester",
		"TOOLEXEC_IMPORTPATH=stale-requester",
	}}
	req.canonicalizeEnviron()
	req.toolexecImportpath = "current-requester"

	env := resolveEnvironment(context.Background(), &req)
	assert.Contains(t, env, "A=value")
	assert.Contains(t, env, envVarParentID+"=current-requester")
	assert.NotContains(t, env, envVarParentID+"=stale-requester")
}
