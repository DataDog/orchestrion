// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

//go:build integration

package gocql

import (
	"context"
	"testing"
	"time"

	"github.com/gocql/gocql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	testcassandra "github.com/testcontainers/testcontainers-go/modules/cassandra"

	"orchestrion/integration/validator/trace"
)

type TestCase struct {
	container *testcassandra.CassandraContainer
	cluster   *gocql.ClusterConfig
	session   *gocql.Session
}

func (tc *TestCase) Setup(t *testing.T) {
	ctx := context.Background()

	var err error
	tc.container, err = testcassandra.Run(ctx,
		"cassandra:4.1",
		testcontainers.WithLogger(testcontainers.TestLogger(t)),
		testcontainers.WithLogConsumers(testLogConsumer{t}),
	)
	require.NoError(t, err)

	host, err := tc.container.ConnectionHost(ctx)
	require.NoError(t, err)

	tc.cluster = gocql.NewCluster(host)
	tc.session, err = tc.cluster.CreateSession()
	require.NoError(t, err)
}

func (tc *TestCase) Run(t *testing.T) {
	err := tc.session.Query("CREATE KEYSPACE if not exists trace WITH REPLICATION = { 'class' : 'SimpleStrategy', 'replication_factor': 1}").Exec()
	require.NoError(t, err)
	err = tc.session.Query("CREATE TABLE if not exists trace.person (name text PRIMARY KEY, age int, description text)").Exec()
	require.NoError(t, err)
	err = tc.session.Query("INSERT INTO trace.person (name, age, description) VALUES ('Cassandra', 100, 'A cruel mistress')").Exec()
	require.NoError(t, err)
}

func (tc *TestCase) Teardown(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	tc.session.Close()
	assert.NoError(t, tc.container.Terminate(ctx))
}

func (*TestCase) ExpectedTraces() trace.Spans {
	return trace.Spans{
		{
			Tags: map[string]any{
				"name": "test.root",
			},
			Children: trace.Spans{
				{
					Tags: map[string]any{
						"name":     "mongodb.query",
						"service":  "mongo",
						"resource": "mongo.insert",
						"type":     "mongodb",
					},
					Meta: map[string]any{
						"component": "go.mongodb.org/mongo-driver/mongo",
						"span.kind": "client",
						"db.system": "mongodb",
					},
				},
				{
					Tags: map[string]any{
						"name":     "mongodb.query",
						"service":  "mongo",
						"resource": "mongo.find",
						"type":     "mongodb",
					},
					Meta: map[string]any{
						"component": "go.mongodb.org/mongo-driver/mongo",
						"span.kind": "client",
						"db.system": "mongodb",
					},
				},
			},
		},
	}
}

type testLogConsumer struct {
	*testing.T
}

func (t testLogConsumer) Accept(log testcontainers.Log) {
	t.T.Log(string(log.Content))
}
