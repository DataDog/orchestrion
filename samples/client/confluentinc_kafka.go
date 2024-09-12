// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"github.com/confluentinc/confluent-kafka-go/kafka"
)

func confluentincKafkaConsumer() {
	cfg := &kafka.ConfigMap{
		"group.id":          "gotest",
		"bootstrap.servers": "localhost:9092",
	}
	consumer, err := kafka.NewConsumer(cfg)
	if err != nil {
		panic(err)
	}
	defer consumer.Close()
}
