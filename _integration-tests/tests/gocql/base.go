// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

//go:build integration

package gocql

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/gocql/gocql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	testcassandra "github.com/testcontainers/testcontainers-go/modules/cassandra"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"

	"orchestrion/integration/utils"
	"orchestrion/integration/validator/trace"
)

type base struct {
	container *testcassandra.CassandraContainer
	session   *gocql.Session
	hostPort  string
	port      string
}

func (b *base) setup(t *testing.T) {
	ctx := context.Background()

	var err error
	b.container, err = testcassandra.Run(ctx,
		"cassandra:4.1",
		testcontainers.WithLogger(testcontainers.TestLogger(t)),
		utils.WithTestLogConsumer(t),
	)
	utils.AssertTestContainersError(t, err)

	b.hostPort, err = b.container.ConnectionHost(ctx)
	require.NoError(t, err)

	_, b.port, err = net.SplitHostPort(b.hostPort)
	require.NoError(t, err)
}

func (b *base) teardown(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	b.session.Close()
	assert.NoError(t, b.container.Terminate(ctx))
}

func (b *base) run(t *testing.T) {
	span, ctx := tracer.StartSpanFromContext(context.Background(), "test.root")
	defer span.Finish()

	err := b.session.
		Query("CREATE KEYSPACE if not exists trace WITH REPLICATION = { 'class' : 'SimpleStrategy', 'replication_factor': 1}").
		WithContext(ctx).
		Exec()
	require.NoError(t, err)

	err = b.session.
		Query("CREATE TABLE if not exists trace.person (name text PRIMARY KEY, age int, description text)").
		WithContext(ctx).
		Exec()
	require.NoError(t, err)

	err = b.session.
		Query("INSERT INTO trace.person (name, age, description) VALUES ('Cassandra', 100, 'A cruel mistress')").
		WithContext(ctx).
		Exec()
	require.NoError(t, err)
}

func (b *base) expectedSpans() trace.Spans {
	return trace.Spans{
		{
			Tags: map[string]any{
				"name": "test.root",
			},
			Children: trace.Spans{
				{
					Tags: map[string]any{
						"name":     "cassandra.query",
						"service":  "gocql.query",
						"resource": "CREATE KEYSPACE if not exists trace WITH REPLICATION = { 'class' : 'SimpleStrategy', 'replication_factor': 1}",
						"type":     "cassandra",
					},
					Meta: map[string]string{
						"component":            "gocql/gocql",
						"span.kind":            "client",
						"db.system":            "cassandra",
						"out.port":             b.port,
						"cassandra.cluster":    "Test Cluster",
						"cassandra.datacenter": "datacenter1",
					},
				},
				{
					Tags: map[string]any{
						"name":     "cassandra.query",
						"service":  "gocql.query",
						"resource": "CREATE TABLE if not exists trace.person (name text PRIMARY KEY, age int, description text)",
						"type":     "cassandra",
					},
					Meta: map[string]string{
						"component":            "gocql/gocql",
						"span.kind":            "client",
						"db.system":            "cassandra",
						"out.port":             b.port,
						"cassandra.cluster":    "Test Cluster",
						"cassandra.datacenter": "datacenter1",
					},
				},
				{
					Tags: map[string]any{
						"name":     "cassandra.query",
						"service":  "gocql.query",
						"resource": "INSERT INTO trace.person (name, age, description) VALUES ('Cassandra', 100, 'A cruel mistress')",
						"type":     "cassandra",
					},
					Meta: map[string]string{
						"component":            "gocql/gocql",
						"span.kind":            "client",
						"db.system":            "cassandra",
						"out.port":             b.port,
						"cassandra.cluster":    "Test Cluster",
						"cassandra.datacenter": "datacenter1",
					},
				},
			},
		},
	}
}
