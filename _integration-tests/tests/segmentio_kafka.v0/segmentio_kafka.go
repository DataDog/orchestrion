// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package segmentio_kafka_v0

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	kafkatest "github.com/testcontainers/testcontainers-go/modules/kafka"

	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"

	"datadoghq.dev/orchestrion/_integration-tests/utils"
	"datadoghq.dev/orchestrion/_integration-tests/validator/trace"
)

const (
	topicA        = "topic-A"
	topicB        = "topic-B"
	consumerGroup = "group-A"
)

type TestCase struct {
	kafka  *kafkatest.KafkaContainer
	addr   string
	writer *kafka.Writer
}

func (tc *TestCase) Setup(t *testing.T) {
	utils.SkipIfProviderIsNotHealthy(t)

	tc.kafka, tc.addr = utils.StartKafkaTestContainer(t)

	tc.writer = &kafka.Writer{
		Addr:     kafka.TCP(tc.addr),
		Balancer: &kafka.LeastBytes{},
	}
}

func (tc *TestCase) newReader(topic string) *kafka.Reader {
	return kafka.NewReader(kafka.ReaderConfig{
		Brokers:  []string{tc.addr},
		GroupID:  consumerGroup,
		Topic:    topic,
		MaxWait:  10 * time.Millisecond,
		MaxBytes: 10e6, // 10MB
	})
}

func (tc *TestCase) Run(t *testing.T) {
	tc.produce(t)
	tc.consume(t)
}

func (tc *TestCase) produce(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	span, ctx := tracer.StartSpanFromContext(ctx, "test.root")
	defer span.Finish()

	messages := []kafka.Message{
		{
			Topic: topicA,
			Key:   []byte("Key-A"),
			Value: []byte("Hello World!"),
		},
		{
			Topic: topicB,
			Key:   []byte("Key-A"),
			Value: []byte("Second message"),
		},
		{
			Topic: topicB,
			Key:   []byte("Key-A"),
			Value: []byte("Third message"),
		},
	}
	const (
		maxRetries = 10
		retryDelay = 100 * time.Millisecond
	)
	var (
		retryCount int
		err        error
	)
	for retryCount < maxRetries {
		err = tc.writer.WriteMessages(ctx, messages...)
		if err == nil {
			break
		}
		// This error happens sometimes with brand-new topics, as there is a delay between when the topic is created
		// on the broker, and when the topic can actually be written to.
		if errors.Is(err, kafka.UnknownTopicOrPartition) {
			retryCount++
			t.Logf("failed to produce kafka messages, will retry in %s (retryCount: %d)", retryDelay, retryCount)
			time.Sleep(retryDelay)
		}
	}
	require.NoError(t, err)
	require.NoError(t, tc.writer.Close())
}

func (tc *TestCase) consume(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	readerA := tc.newReader(topicA)
	m, err := readerA.ReadMessage(ctx)
	require.NoError(t, err)
	assert.Equal(t, "Hello World!", string(m.Value))
	assert.Equal(t, "Key-A", string(m.Key))
	require.NoError(t, readerA.Close())

	readerB := tc.newReader(topicB)
	m, err = readerB.FetchMessage(ctx)
	require.NoError(t, err)
	assert.Equal(t, "Second message", string(m.Value))
	assert.Equal(t, "Key-A", string(m.Key))
	err = readerB.CommitMessages(ctx, m)
	require.NoError(t, err)
	require.NoError(t, readerB.Close())
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
						"name":     "kafka.produce",
						"type":     "queue",
						"service":  "kafka",
						"resource": "Produce Topic topic-A",
					},
					Meta: map[string]string{
						"span.kind": "producer",
						"component": "segmentio/kafka.go.v0",
					},
					Children: trace.Traces{
						{
							Tags: map[string]any{
								"name":     "kafka.consume",
								"type":     "queue",
								"service":  "kafka",
								"resource": "Consume Topic topic-A",
							},
							Meta: map[string]string{
								"span.kind": "consumer",
								"component": "segmentio/kafka.go.v0",
							},
						},
					},
				},
				{
					Tags: map[string]any{
						"name":     "kafka.produce",
						"type":     "queue",
						"service":  "kafka",
						"resource": "Produce Topic topic-B",
					},
					Meta: map[string]string{
						"span.kind": "producer",
						"component": "segmentio/kafka.go.v0",
					},
					Children: trace.Traces{
						{
							Tags: map[string]any{
								"name":     "kafka.consume",
								"type":     "queue",
								"service":  "kafka",
								"resource": "Consume Topic topic-B",
							},
							Meta: map[string]string{
								"span.kind": "consumer",
								"component": "segmentio/kafka.go.v0",
							},
						},
					},
				},
				{
					Tags: map[string]any{
						"name":     "kafka.produce",
						"type":     "queue",
						"service":  "kafka",
						"resource": "Produce Topic topic-B",
					},
					Meta: map[string]string{
						"span.kind": "producer",
						"component": "segmentio/kafka.go.v0",
					},
					Children: nil,
				},
			},
		},
	}
}
