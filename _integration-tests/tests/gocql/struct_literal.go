// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

//go:build integration

package gocql

import (
	"testing"
	"time"

	"github.com/gocql/gocql"
	"github.com/stretchr/testify/require"

	"orchestrion/integration/validator/trace"
)

type TestCaseStructLiteral struct {
	base
}

func (tc *TestCaseStructLiteral) Setup(t *testing.T) {
	tc.setup(t)

	var err error
	cluster := gocql.ClusterConfig{
		Hosts:                  []string{tc.hostPort},
		CQLVersion:             "3.0.0",
		Timeout:                11 * time.Second,
		ConnectTimeout:         11 * time.Second,
		NumConns:               2,
		Consistency:            gocql.Quorum,
		MaxPreparedStmts:       1000,
		MaxRoutingKeyInfo:      1000,
		PageSize:               5000,
		DefaultTimestamp:       true,
		MaxWaitSchemaAgreement: 60 * time.Second,
		ReconnectInterval:      60 * time.Second,
		ConvictionPolicy:       &gocql.SimpleConvictionPolicy{},
		ReconnectionPolicy:     &gocql.ConstantReconnectionPolicy{MaxRetries: 3, Interval: 1 * time.Second},
		WriteCoalesceWaitTime:  200 * time.Microsecond,
	}
	tc.session, err = cluster.CreateSession()
	require.NoError(t, err)
}

func (tc *TestCaseStructLiteral) Run(t *testing.T) {
	tc.base.run(t)
}

func (tc *TestCaseStructLiteral) Teardown(t *testing.T) {
	tc.base.teardown(t)
}

func (tc *TestCaseStructLiteral) ExpectedTraces() trace.Spans {
	return tc.base.expectedSpans()
}
