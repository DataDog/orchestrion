// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package mongo

import (
	"context"
	"log"
	"net/url"
	"orchestrion/integration/validator/trace"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	testmongo "github.com/testcontainers/testcontainers-go/modules/mongodb"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type TestCase struct {
	server *testmongo.MongoDBContainer
	*mongo.Client
}

func (tc *TestCase) Setup(t *testing.T) {
	ctx := context.Background()

	var err error
	tc.server, err = testmongo.Run(ctx,
		"mongo:6",
		testcontainers.WithLogger(testcontainers.TestLogger(t)),
		testcontainers.WithLogConsumers(testLogConsumer{t}),
	)
	if err != nil {
		t.Skipf("Failed to start mongo test container: %v\n", err)
	}

	mongoURI, err := tc.server.ConnectionString(ctx)
	if err != nil {
		log.Fatalf("Failed to obtain connection string: %v\n", err)
	}
	_, err = url.Parse(mongoURI)
	if err != nil {
		log.Fatalf("Invalid mongo connection string: %q\n", mongoURI)
	}
	opts := options.Client()
	opts.ApplyURI(mongoURI)
	client, err := mongo.Connect(context.Background(), opts)
	if err != nil {
		log.Fatalf("Failed to connect to mongo: %v\n", err)
	}
	tc.Client = client
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

func (tc *TestCase) Teardown(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	assert.NoError(t, tc.Client.Disconnect(ctx))
	assert.NoError(t, tc.server.Terminate(ctx))
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
						"name":     "mongo.command",
						"service":  "mongo.client",
						"resource": "set",
						"type":     "mongo",
					},
					Meta: map[string]any{
						"mongo.args_length": "3",
						"component":         "go-mongo/mongo.v8",
						"out.db":            "0",
						"span.kind":         "client",
						"db.system":         "mongo",
						"mongo.raw_command": "set test_key test_value:",
						"out.host":          "localhost",
					},
				},
				{
					Tags: map[string]any{
						"name":     "mongo.command",
						"service":  "mongo.client",
						"resource": "get",
						"type":     "mongo",
					},
					Meta: map[string]any{
						"mongo.args_length": "2",
						"component":         "go-mongo/mongo.v8",
						"out.db":            "0",
						"span.kind":         "client",
						"db.system":         "mongo",
						"mongo.raw_command": "get test_key:",
						"out.host":          "localhost",
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
