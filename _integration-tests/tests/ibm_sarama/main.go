// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package ibm_sarama

import (
	"context"
	"fmt"
	"testing"
	"time"

	"orchestrion/integration/utils"
	"orchestrion/integration/validator/trace"

	"github.com/IBM/sarama"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	testkafka "github.com/testcontainers/testcontainers-go/modules/kafka"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	topic     = "gotest"
	partition = int32(0)
)

type TestCase struct {
	server *testkafka.KafkaContainer
	cfg    *sarama.Config
	addrs  []string
}

func (tc *TestCase) Setup(t *testing.T) {
	var (
		err error
		ctx = context.Background()
	)
	tc.cfg = sarama.NewConfig()
	tc.cfg.Version = sarama.V0_11_0_0
	tc.cfg.Producer.Return.Successes = true

	tc.server, err = testkafka.Run(ctx,
		"confluentinc/confluent-local:7.5.0",
		testkafka.WithClusterID("test-cluster"),
		testcontainers.WithWaitStrategy(wait.ForListeningPort("9093/tcp")),
		testcontainers.WithLogger(testcontainers.TestLogger(t)),
		utils.WithTestLogConsumer(t),
	)
	if err != nil {
		t.Skipf("Failed to start kafka test container: %v\n", err)
	}

	state, err := tc.server.State(ctx)
	require.NoError(t, err)

	if !state.Running {
		t.Skip("Failed to start kafka test container")
	}

	addr, err := tc.server.Host(ctx)
	require.NoError(t, err)
	port, err := tc.server.MappedPort(ctx, "9093/tcp")
	require.NoError(t, err)
	tc.addrs = []string{fmt.Sprintf("%s:%s", addr, port.Port())}
}

func produceMessage(t *testing.T, addrs []string, cfg *sarama.Config) {
	t.Helper()

	producer, err := sarama.NewSyncProducer(addrs, cfg)
	if err != nil {
		t.Fatal(err)
	}

	producer.SendMessage(&sarama.ProducerMessage{
		Topic:     topic,
		Partition: partition,
		Value:     sarama.StringEncoder("Hello, World!"),
	})
}

func consumeMessage(t *testing.T, addrs []string, cfg *sarama.Config) {
	t.Helper()

	consumer, err := sarama.NewConsumer(addrs, cfg)
	require.NoError(t, err)
	defer consumer.Close()

	partitionConsumer, err := consumer.ConsumePartition(topic, partition, sarama.OffsetOldest)
	require.NoError(t, err)
	defer partitionConsumer.Close()

	select {
	case msg := <-partitionConsumer.Messages():
		require.Equal(t, "Hello, World!", string(msg.Value))
	case <-time.After(15 * time.Second):
		t.Fatal("timed out waiting for message")
	}
}

func (tc *TestCase) Run(t *testing.T) {
	produceMessage(t, tc.addrs, tc.cfg)
	consumeMessage(t, tc.addrs, tc.cfg)
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
				"name":    "kafka.produce",
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
						"name":    "kafka.consume",
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
