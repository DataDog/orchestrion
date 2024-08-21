// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import "github.com/Shopify/sarama"

func shopifySaramaConsumer() {
	cfg := sarama.NewConfig()
	consumer, err := sarama.NewConsumer([]string{"localhost:9092"}, cfg)
	if err != nil {
		panic(err)
	}
	defer consumer.Close()
}

func shopifySaramaConsumerFromClient() {
	client, err := sarama.NewClient([]string{"localhost:9092"}, nil)
	if err != nil {
		panic(err)
	}
	consumer, err := sarama.NewConsumerFromClient(client)
	if err != nil {
		panic(err)
	}
	defer consumer.Close()
}
