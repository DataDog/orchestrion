// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"context"
	"log"
	"time"

	"cloud.google.com/go/pubsub"
)

const (
	projectID        = "project-id"
	topicName        = "topic-name"
	subscriptionName = "subscription-name"
)

func SampleGCPPubsub() {
	ctx := context.Background()
	client, err := pubsub.NewClient(ctx, projectID)
	if err != nil {
		log.Fatal(err)
	}

	topic, err := client.CreateTopic(ctx, topicName)
	if err != nil {
		log.Fatal(err)
	}

	_, err = client.CreateSubscription(ctx, subscriptionName, pubsub.SubscriptionConfig{Topic: topic})
	if err != nil {
		log.Fatal(err)
	}

}

func publishMessage(client *pubsub.Client) {
	ctx := context.Background()
	topic := client.Topic(topicName)
	res := topic.Publish(context.Background(), &pubsub.Message{
		Data: []byte("Hello, World!"),
	})
	_, err := res.Get(ctx)
	if err != nil {
		log.Fatal(err)
	}
}

func receiveMessage(client *pubsub.Client) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sub := client.Subscription(subscriptionName)
	err := sub.Receive(ctx, func(ctx context.Context, message *pubsub.Message) {
		log.Printf("got message: %s", message.Data)
		message.Ack()
		cancel()
	})
	if err != nil {
		log.Fatal(err)
	}

	<-ctx.Done()
}
