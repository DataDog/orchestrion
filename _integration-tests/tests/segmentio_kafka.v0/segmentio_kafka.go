// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package segmentio_kafka_v0

import (
	"context"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	kafkatest "github.com/testcontainers/testcontainers-go/modules/kafka"

	"orchestrion/integration/utils"
	"orchestrion/integration/validator/trace"
)

const (
	topic         = "topic-A"
	consumerGroup = "group-A"
)

type TestCase struct {
	kafka  *kafkatest.KafkaContainer
	addr   string
	writer *kafka.Writer
	reader *kafka.Reader
}

func (tc *TestCase) Setup(t *testing.T) {
	tc.kafka, tc.addr = utils.StartKafkaTestContainer(t)

	tc.writer = &kafka.Writer{
		Addr:     kafka.TCP(tc.addr),
		Topic:    topic,
		Balancer: &kafka.LeastBytes{},
	}
	tc.reader = kafka.NewReader(kafka.ReaderConfig{
		Brokers:  []string{tc.addr},
		GroupID:  consumerGroup,
		Topic:    topic,
		MaxBytes: 10e6, // 10MB
	})

	tc.createTopic(t)
}

func (tc *TestCase) createTopic(t *testing.T) {
	conn, err := kafka.Dial("tcp", tc.addr)
	require.NoError(t, err)
	defer conn.Close()

	controller, err := conn.Controller()
	require.NoError(t, err)

	controllerConn, err := kafka.Dial("tcp", net.JoinHostPort(controller.Host, strconv.Itoa(controller.Port)))
	require.NoError(t, err)
	defer controllerConn.Close()

	topicConfigs := []kafka.TopicConfig{
		{
			Topic:             topic,
			NumPartitions:     1,
			ReplicationFactor: 1,
		},
	}
	err = controllerConn.CreateTopics(topicConfigs...)
	require.NoError(t, err)
}

func (tc *TestCase) Run(t *testing.T) {
	tc.produce(t)
	tc.consume(t)
}

func (tc *TestCase) produce(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := tc.writer.WriteMessages(ctx,
		kafka.Message{
			Key:   []byte("Key-A"),
			Value: []byte("Hello World!"),
		},
	)
	require.NoError(t, err)
}

func (tc *TestCase) consume(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	m, err := tc.reader.ReadMessage(ctx)
	require.NoError(t, err)
	assert.Equal(t, "Hello World!", string(m.Value))
	assert.Equal(t, "Key-A", string(m.Key))
}

func (tc *TestCase) Teardown(t *testing.T) {
	require.NoError(t, tc.writer.Close())
	require.NoError(t, tc.reader.Close())

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	require.NoError(t, tc.kafka.Terminate(ctx))
}

func (tc *TestCase) ExpectedTraces() trace.Spans {
	return trace.Spans{
		{
			Tags: map[string]any{
				"name":     "kafka.produce",
				"type":     "queue",
				"service":  "kafka",
				"resource": "Produce Topic topic-A",
			},
			Meta: map[string]any{
				"span.kind": "producer",
				"component": "segmentio/kafka.go.v0",
			},
			Children: trace.Spans{
				{
					Tags: map[string]any{
						"name":     "kafka.consume",
						"type":     "queue",
						"service":  "kafka",
						"resource": "Consume Topic topic-A",
					},
					Meta: map[string]any{
						"span.kind": "consumer",
						"component": "segmentio/kafka.go.v0",
					},
				},
			},
		},
	}
}
