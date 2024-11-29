// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

//go:build integration

package gcppubsub

import (
	"context"
	"testing"
	"time"

	"cloud.google.com/go/pubsub"
	"datadoghq.dev/orchestrion/_integration-tests/utils"
	"datadoghq.dev/orchestrion/_integration-tests/validator/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/gcloud"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	testTopic        = "pstest-orchestrion-topic"
	testSubscription = "pstest-orchestrion-subscription"
)

type TestCase struct {
	container   *gcloud.GCloudContainer
	client      *pubsub.Client
	publishTime time.Time
	messageID   string
}

func (tc *TestCase) Setup(t *testing.T) {
	utils.SkipIfProviderIsNotHealthy(t)

	var (
		err error
		ctx = context.Background()
	)

	tc.container, err = gcloud.RunPubsub(ctx,
		"gcr.io/google.com/cloudsdktool/google-cloud-cli:emulators",
		gcloud.WithProjectID("pstest-orchestrion"),
		testcontainers.WithLogger(testcontainers.TestLogger(t)),
		utils.WithTestLogConsumer(t),
	)
	utils.AssertTestContainersError(t, err)
	utils.RegisterContainerCleanup(t, tc.container)

	projectID := tc.container.Settings.ProjectID

	//orchestrion:ignore
	conn, err := grpc.NewClient(tc.container.URI, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)

	tc.client, err = pubsub.NewClient(ctx, projectID, option.WithGRPCConn(conn))
	require.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, tc.client.Close())
	})

	topic, err := tc.client.CreateTopic(ctx, testTopic)
	require.NoError(t, err)

	_, err = tc.client.CreateSubscription(ctx, testSubscription, pubsub.SubscriptionConfig{
		Topic:                 topic,
		EnableMessageOrdering: true,
	})
	require.NoError(t, err)
}

func (tc *TestCase) publishMessage(t *testing.T) {
	t.Helper()

	ctx := context.Background()
	topic := tc.client.Topic(testTopic)
	topic.EnableMessageOrdering = true
	res := topic.Publish(context.Background(), &pubsub.Message{
		Data:        []byte("Hello, World!"),
		OrderingKey: "ordering-key",
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
	err := sub.Receive(ctx, func(_ context.Context, message *pubsub.Message) {
		assert.Equal(t, message.Data, []byte("Hello, World!"))
		message.Ack()
		tc.publishTime = message.PublishTime
		tc.messageID = message.ID
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

func (tc *TestCase) ExpectedTraces() trace.Traces {
	return trace.Traces{
		{
			Tags: map[string]any{
				"name":     "pubsub.publish",
				"type":     "queue",
				"resource": "projects/pstest-orchestrion/topics/pstest-orchestrion-topic",
				"service":  "gcp_pubsub.test",
			},
			Meta: map[string]string{
				"span.kind":    "producer",
				"component":    "cloud.google.com/go/pubsub.v1",
				"ordering_key": "ordering-key",
			},
			Children: trace.Traces{
				{
					Tags: map[string]any{
						"name":     "pubsub.receive",
						"type":     "queue",
						"resource": "projects/pstest-orchestrion/subscriptions/pstest-orchestrion-subscription",
						"service":  "gcp_pubsub.test",
					},
					Meta: map[string]string{
						"span.kind":        "consumer",
						"component":        "cloud.google.com/go/pubsub.v1",
						"messaging.system": "googlepubsub",
						"ordering_key":     "ordering-key",
						"publish_time":     tc.publishTime.String(),
						"message_id":       tc.messageID,
					},
				},
			},
		},
	}
}
