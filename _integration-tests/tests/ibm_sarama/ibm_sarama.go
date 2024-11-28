// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

//go:build integration && (linux || !githubci)

package ibm_sarama

import (
	"fmt"
	"testing"
	"time"

	"datadoghq.dev/orchestrion/_integration-tests/utils"
	"datadoghq.dev/orchestrion/_integration-tests/validator/trace"
	"github.com/IBM/sarama"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/kafka"
)

const (
	topic     = "gotest"
	partition = int32(0)
)

type TestCase struct {
	server *kafka.KafkaContainer
	cfg    *sarama.Config
	addrs  []string
}

func (tc *TestCase) Setup(t *testing.T) {
	utils.SkipIfProviderIsNotHealthy(t)

	tc.cfg = sarama.NewConfig()
	tc.cfg.Version = sarama.V0_11_0_0
	tc.cfg.Producer.Return.Successes = true

	container, addr := utils.StartKafkaTestContainer(t)
	tc.server = container
	tc.addrs = []string{addr}
}

func produceMessage(t *testing.T, addrs []string, cfg *sarama.Config) {
	t.Helper()

	createProducer := func() (_ sarama.SyncProducer, err error) {
		defer func() {
			if r := recover(); r != nil && err == nil {
				var ok bool
				if err, ok = r.(error); !ok {
					err = fmt.Errorf("panic: %v", r)
				}
			}
		}()
		return sarama.NewSyncProducer(addrs, cfg)
	}

	var (
		producer sarama.SyncProducer
		err      error
	)
	for attemptsLeft := 3; attemptsLeft > 0; attemptsLeft-- {
		producer, err = createProducer()
		if err != nil {
			time.Sleep(50 * time.Millisecond)
			continue
		}
		break
	}
	require.NoError(t, err, "failed to create producer")
	defer func() { assert.NoError(t, producer.Close(), "failed to close producer") }()

	_, _, err = producer.SendMessage(&sarama.ProducerMessage{
		Topic:     topic,
		Partition: partition,
		Value:     sarama.StringEncoder("Hello, World!"),
	})
	require.NoError(t, err, "failed to send message")
	_, _, err = producer.SendMessage(&sarama.ProducerMessage{
		Topic:     topic,
		Partition: partition,
		Value:     sarama.StringEncoder("Another message to avoid flaky tests"),
	})
	require.NoError(t, err, "failed to send message")
}

func consumeMessage(t *testing.T, addrs []string, cfg *sarama.Config) {
	t.Helper()

	consumer, err := sarama.NewConsumer(addrs, cfg)
	require.NoError(t, err, "failed to create consumer")
	defer func() { assert.NoError(t, consumer.Close(), "failed to close consumer") }()

	partitionConsumer, err := consumer.ConsumePartition(topic, partition, sarama.OffsetOldest)
	require.NoError(t, err, "failed to create partition consumer")
	defer func() { assert.NoError(t, partitionConsumer.Close(), "failed to close partition consumer") }()

	expectedMessages := []string{"Hello, World!", "Another message to avoid flaky tests"}
	for i := 0; i < len(expectedMessages); i++ {
		select {
		case msg := <-partitionConsumer.Messages():
			require.Equal(t, expectedMessages[i], string(msg.Value))
		case <-time.After(15 * time.Second):
			t.Fatal("timed out waiting for message")
		}
	}
}

func (tc *TestCase) Run(t *testing.T) {
	produceMessage(t, tc.addrs, tc.cfg)
	consumeMessage(t, tc.addrs, tc.cfg)
}

func (*TestCase) ExpectedTraces() trace.Traces {
	return trace.Traces{
		{
			Tags: map[string]any{
				"name":    "kafka.produce",
				"type":    "queue",
				"service": "kafka",
			},
			Meta: map[string]string{
				"span.kind": "producer",
				"component": "IBM/sarama",
			},
			Children: trace.Traces{
				{
					Tags: map[string]any{
						"name":    "kafka.consume",
						"type":    "queue",
						"service": "kafka",
					},
					Meta: map[string]string{
						"span.kind": "consumer",
						"component": "IBM/sarama",
					},
				},
			},
		},
	}
}
