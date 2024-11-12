// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

//go:build integration

package mongo

import (
	"context"
	"net/url"
	"testing"
	"time"

	"datadoghq.dev/orchestrion/_integration-tests/utils"
	"datadoghq.dev/orchestrion/_integration-tests/validator/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	testmongo "github.com/testcontainers/testcontainers-go/modules/mongodb"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
)

type TestCase struct {
	server *testmongo.MongoDBContainer
	*mongo.Client
}

func (tc *TestCase) Setup(t *testing.T) {
	utils.SkipIfProviderIsNotHealthy(t)

	ctx := context.Background()

	var err error
	tc.server, err = testmongo.Run(ctx,
		"mongo:6",
		testcontainers.WithLogger(testcontainers.TestLogger(t)),
		utils.WithTestLogConsumer(t),
	)
	utils.AssertTestContainersError(t, err)
	utils.RegisterContainerCleanup(t, tc.server)

	mongoURI, err := tc.server.ConnectionString(ctx)
	require.NoError(t, err)
	_, err = url.Parse(mongoURI)
	require.NoError(t, err)

	opts := options.Client()
	opts.ApplyURI(mongoURI)
	client, err := mongo.Connect(context.Background(), opts)
	require.NoError(t, err)
	tc.Client = client
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		assert.NoError(t, tc.Client.Disconnect(ctx))
	})
}

func (tc *TestCase) Run(t *testing.T) {
	ctx := context.Background()
	span, ctx := tracer.StartSpanFromContext(ctx, "test.root")
	defer span.Finish()

	db := tc.Client.Database("test")
	c := db.Collection("coll")

	_, err := c.InsertOne(ctx, bson.M{"test_key": "test_value"})
	require.NoError(t, err)
	r := c.FindOne(ctx, bson.M{"test_key": "test_value"})
	require.NoError(t, r.Err())
}

func (*TestCase) ExpectedTraces() trace.Traces {
	return trace.Traces{
		{
			Tags: map[string]any{
				"name": "test.root",
			},
			Children: trace.Traces{
				{
					Tags: map[string]any{
						"name":     "mongodb.query",
						"service":  "mongo",
						"resource": "mongo.insert",
						"type":     "mongodb",
					},
					Meta: map[string]string{
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
					Meta: map[string]string{
						"component": "go.mongodb.org/mongo-driver/mongo",
						"span.kind": "client",
						"db.system": "mongodb",
					},
				},
			},
		},
	}
}
