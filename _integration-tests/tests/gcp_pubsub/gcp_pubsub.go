// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

//go:build integration || gcp_pubsub

package gcp_pubsub

import (
	"context"
	"testing"
	"time"

	"cloud.google.com/go/pubsub"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/gcloud"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"orchestrion/integration/utils"
	"orchestrion/integration/validator/trace"
)

const (
	testTopic        = "pstest-orchestrion-topic"
	testSubscription = "pstest-orchestrion-subscription"
)

type TestCase struct {
	container *gcloud.GCloudContainer
	client    *pubsub.Client
}

func (tc *TestCase) Setup(t *testing.T) {
	var (
		err error
		ctx = context.Background()
	)

	tc.container, err = gcloud.RunPubsub(ctx,
		"gcr.io/google.com/cloudsdktool/cloud-sdk:490.0.0-emulators",
		gcloud.WithProjectID("pstest-orchestrion"),
		testcontainers.WithLogger(testcontainers.TestLogger(t)),
		utils.WithTestLogConsumer(t),
	)
	utils.AssertTestContainersError(t, err)

	projectID := tc.container.Settings.ProjectID

	//dd:ignore
	conn, err := grpc.NewClient(tc.container.URI, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)

	tc.client, err = pubsub.NewClient(ctx, projectID, option.WithGRPCConn(conn))
	require.NoError(t, err)

	topic, err := tc.client.CreateTopic(ctx, testTopic)
	require.NoError(t, err)

	_, err = tc.client.CreateSubscription(ctx, testSubscription, pubsub.SubscriptionConfig{Topic: topic})
	require.NoError(t, err)
}

func (tc *TestCase) publishMessage(t *testing.T) {
	t.Helper()

	ctx := context.Background()
	topic := tc.client.Topic(testTopic)
	res := topic.Publish(context.Background(), &pubsub.Message{
		Data: []byte("Hello, World!"),
	})
	_, err := res.Get(ctx)
	require.NoError(t, err)
	t.Log("finished publishing result")
}

func (tc *TestCase) receiveMessage(t *testing.T) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sub := tc.client.Subscription(testSubscription)
	err := sub.Receive(ctx, func(ctx context.Context, message *pubsub.Message) {
		assert.Equal(t, message.Data, []byte("Hello, World!"))
		message.Ack()
		cancel()
	})
	require.NoError(t, err)

	<-ctx.Done()
	require.NotErrorIs(t, ctx.Err(), context.DeadlineExceeded)
}

func (tc *TestCase) Run(t *testing.T) {
	tc.publishMessage(t)
	tc.receiveMessage(t)
}

func (tc *TestCase) Teardown(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	require.NoError(t, tc.client.Close())
	require.NoError(t, tc.container.Terminate(ctx))
}

func (tc *TestCase) ExpectedTraces() trace.Spans {
	return trace.Spans{
		{
			Tags: map[string]any{
				"name":    "pubsub.publish",
				"type":    "queue",
				"service": "kafka",
			},
			Meta: map[string]any{
				"span.kind": "producer",
				"component": "IBM/sarama",
			},
			Children: trace.Spans{
				{
					Tags: map[string]any{
						"name":    "pubsub.receive",
						"type":    "queue",
						"service": "kafka",
					},
					Meta: map[string]any{
						"span.kind": "consumer",
						"component": "IBM/sarama",
					},
				},
			},
		},
	}
}
