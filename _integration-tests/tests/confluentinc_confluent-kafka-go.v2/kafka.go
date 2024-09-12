// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

//go:build integration

package kafka

import (
	"context"
	"strings"
	"testing"
	"time"

	"orchestrion/integration/utils"
	"orchestrion/integration/validator/trace"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/redpanda"
)

var (
	topic     = "gotest"
	partition = int32(0)
)

type TestCase struct {
	server *redpanda.Container
	addr   []string
}

func (tc *TestCase) Setup(t *testing.T) {
	var (
		err error
		ctx = context.Background()
	)

	tc.server, err = redpanda.Run(ctx,
		"docker.redpanda.com/redpandadata/redpanda:v24.2.1",
		redpanda.WithAutoCreateTopics(),
		testcontainers.WithLogger(testcontainers.TestLogger(t)),
		utils.WithTestLogConsumer(t),
	)
	if err != nil {
		t.Skipf("Failed to start kafka test container: %v\n", err)
	}

	addr, err := tc.server.KafkaSeedBroker(ctx)
	require.NoError(t, err, "failed to get seed broker address")

	tc.addr = []string{addr}
}

func (tc *TestCase) Run(t *testing.T) {
	produceMessage(t, &kafka.ConfigMap{
		"group.id":            "gotest",
		"bootstrap.servers":   strings.Join(tc.addr, ","),
		"go.delivery.reports": true,
	})
	consumeMessage(t, &kafka.ConfigMap{
		"group.id":          "gotest",
		"bootstrap.servers": strings.Join(tc.addr, ","),
	})
}

func produceMessage(t *testing.T, cfg *kafka.ConfigMap) {
	t.Helper()
	delivery := make(chan kafka.Event, 1)

	producer, err := kafka.NewProducer(cfg)
	require.NoError(t, err, "failed to create producer")
	defer func() {
		<-delivery
		producer.Close()
	}()

	err = producer.Produce(&kafka.Message{
		TopicPartition: kafka.TopicPartition{
			Topic:     &topic,
			Partition: partition,
		},
		Key:   []byte("key2"),
		Value: []byte("value2"),
	}, delivery)
	require.NoError(t, err, "failed to send message")
}

func consumeMessage(t *testing.T, cfg *kafka.ConfigMap) {
	t.Helper()

	c, err := kafka.NewConsumer(cfg)
	require.NoError(t, err, "failed to create consumer")
	defer c.Close()

	err = c.Assign([]kafka.TopicPartition{
		{Topic: &topic, Partition: 0},
	})
	require.NoError(t, err)

	m, err := c.ReadMessage(3000 * time.Millisecond)
	require.NoError(t, err)

	_, err = c.CommitMessage(m)
	require.NoError(t, err)

	require.Equal(t, "key2", string(m.Key))
}

func (tc *TestCase) Teardown(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	require.NoError(t, tc.server.Terminate(ctx))
}

func (*TestCase) ExpectedTraces() trace.Spans {
	return trace.Spans{
		{
			Tags: map[string]interface{}{
				"service": "kafka",
				"name":    "kafka.produce",
				"type":    "queue",
			},
			Meta: map[string]interface{}{
				"component":        "confluentinc/confluent-kafka-go/kafka.v2",
				"messaging.system": "kafka",
				"span.kind":        "producer",
			},
			Children: []*trace.Span{
				{
					Tags: map[string]interface{}{
						"service": "kafka",
						"name":    "kafka.consume",
						"type":    "queue",
					},
					Meta: map[string]interface{}{
						"component":        "confluentinc/confluent-kafka-go/kafka.v2",
						"messaging.system": "kafka",
						"span.kind":        "consumer",
					},
				},
			},
		},
	}
}
