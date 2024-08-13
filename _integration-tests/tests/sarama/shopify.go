// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package sarama

import (
	"context"
	"testing"
	"time"

	"orchestrion/integration/utils"
	"orchestrion/integration/validator/trace"

	"github.com/Shopify/sarama"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	testkafka "github.com/testcontainers/testcontainers-go/modules/kafka"
)

type TestCase struct {
	server *testkafka.KafkaContainer
	cfg    *sarama.Config
}

func (tc *TestCase) Setup(t *testing.T) {
	ctx := context.Background()

	var err error
	tc.server, err = testkafka.Run(ctx,
		"darccio/kafka:2.13-2.8.1",
		testcontainers.WithLogger(testcontainers.TestLogger(t)),
		utils.WithTestLogConsumer(t),
	)
	if err != nil {
		t.Skipf("Failed to start kafka test container: %v\n", err)
	}
}

func (tc *TestCase) Run(t *testing.T) {
	consumer, err := sarama.NewConsumer([]string{"localhost:9092"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer consumer.Close()

	producer, err := sarama.NewSyncProducer([]string{"localhost:9092"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer producer.Close()

	topic := "test"
	partition := int32(0)
	producer.SendMessage(&sarama.ProducerMessage{
		Topic:     topic,
		Partition: partition,
		Value:     sarama.StringEncoder("Hello, World!"),
	})

	partitionConsumer, err := consumer.ConsumePartition(topic, partition, sarama.OffsetOldest)
	if err != nil {
		t.Fatal(err)
	}
	defer partitionConsumer.Close()

	select {
	case msg := <-partitionConsumer.Messages():
		require.Equal(t, "Hello, World!", string(msg.Value))
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for message")
	}
}

func (tc *TestCase) Teardown(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	require.NoError(t, tc.server.Terminate(ctx))
}

func (tc *TestCase) ExpectedTraces() trace.Spans {
	return trace.Spans{
		{
			Tags: map[string]any{
				"name": "test.root",
			},
		},
	}
}
