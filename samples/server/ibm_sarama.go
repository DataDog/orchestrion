// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import "github.com/IBM/sarama"

func ibmSaramaProducer() {
	cfg := sarama.NewConfig()
	cfg.Producer.Return.Successes = true

	producer, err := sarama.NewSyncProducer([]string{"localhost:9092"}, cfg)
	if err != nil {
		panic(err)
	}
	defer producer.Close()
}

func ibmSaramaProducerFromClient() {
	cfg := sarama.NewConfig()
	cfg.Producer.Return.Successes = true

	client, err := sarama.NewClient([]string{"localhost:9092"}, cfg)
	if err != nil {
		panic(err)
	}
	producer, err := sarama.NewSyncProducerFromClient(client)
	if err != nil {
		panic(err)
	}
	defer producer.Close()
}

func ibmSaramaAsyncProducer() {
	cfg := sarama.NewConfig()
	cfg.Version = sarama.V0_11_0_0 // minimum version that supports headers which are required for tracing

	producer, err := sarama.NewAsyncProducer([]string{"localhost:9092"}, cfg)
	if err != nil {
		panic(err)
	}
	defer producer.Close()
}

func ibmSaramaAsyncProducerFromClient() {
	cfg := sarama.NewConfig()
	cfg.Version = sarama.V0_11_0_0 // minimum version that supports headers which are required for tracing

	client, err := sarama.NewClient([]string{"localhost:9092"}, cfg)
	if err != nil {
		panic(err)
	}
	producer, err := sarama.NewAsyncProducerFromClient(client)
	if err != nil {
		panic(err)
	}
	defer producer.Close()
}
