// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func mongoClient() {
	ctx := context.Background()
	opts := options.Client().ApplyURI("mongodb://localhost:27017")
	client, err := mongo.Connect(ctx, opts)
	if err != nil {
		panic(err)
	}
	defer client.Disconnect(ctx)

	db := client.Database("test")
	c := db.Collection("coll")
	if _, err := c.InsertOne(ctx, bson.M{"key": "value"}); err != nil {
		panic(err)
	}
}
