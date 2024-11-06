// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

//go:build integration

package gocql

import (
	"testing"

	"datadoghq.dev/orchestrion/_integration-tests/validator/trace"
	"github.com/gocql/gocql"
	"github.com/stretchr/testify/require"
)

type TestCaseNewCluster struct {
	base
}

func (tc *TestCaseNewCluster) Setup(t *testing.T) {
	tc.setup(t)

	var err error
	cluster := gocql.NewCluster(tc.hostPort)
	tc.session, err = cluster.CreateSession()
	require.NoError(t, err)
	t.Cleanup(func() {
		tc.session.Close()
	})
}

func (tc *TestCaseNewCluster) Run(t *testing.T) {
	tc.base.run(t)
}

func (tc *TestCaseNewCluster) ExpectedTraces() trace.Traces {
	return tc.base.expectedTraces()
}
