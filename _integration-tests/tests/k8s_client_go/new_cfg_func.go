// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

//go:build integration

package k8sclientgo

import (
	"context"
	"testing"

	"datadoghq.dev/orchestrion/_integration-tests/validator/trace"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

type TestCaseNewCfgFunc struct {
	base
}

func (tc *TestCaseNewCfgFunc) Setup(t *testing.T, ctx context.Context) {
	tc.base.setup(t, ctx)

	// internally, this function creates a rest.Config struct literal, so it should get traced by orchestrion.
	cfg, err := clientcmd.BuildConfigFromKubeconfigGetter(tc.server.URL, func() (*clientcmdapi.Config, error) {
		return clientcmdapi.NewConfig(), nil
	})
	require.NoError(t, err)

	client, err := kubernetes.NewForConfig(cfg)
	require.NoError(t, err)
	tc.base.client = client
}

func (tc *TestCaseNewCfgFunc) Run(t *testing.T, ctx context.Context) {
	tc.base.run(t, ctx)
}

func (tc *TestCaseNewCfgFunc) ExpectedTraces() trace.Traces {
	return tc.base.expectedTraces()
}
