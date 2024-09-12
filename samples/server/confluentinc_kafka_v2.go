// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2024-present Datadog, Inc.

package main

import (
	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
)

func confluentincV2KafkaProducer() {
	cfg := &kafka.ConfigMap{
		"group.id":            "gotest",
		"bootstrap.servers":   "localhost:9092",
		"go.delivery.reports": true,
	}
	producer, err := kafka.NewProducer(cfg)
	if err != nil {
		panic(err)
	}
	defer producer.Close()
}
