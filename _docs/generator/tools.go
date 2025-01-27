// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

//go:build tools

package main

import (
	_ "github.com/DataDog/orchestrion"
	_ "github.com/DataDog/orchestrion/instrument"

	// Packages not directly imported by the integrations, but implied by them,
	// when we therefore need to be able to resolve when generating the
	// documentation...
	_ "cloud.google.com/go/internal/pubsub"
	_ "cloud.google.com/go/pubsub"
	_ "github.com/99designs/gqlgen/graphql/handler"
	_ "github.com/IBM/sarama"
	_ "github.com/Shopify/sarama"
	_ "github.com/aws/aws-sdk-go-v2/aws"
	_ "github.com/aws/aws-sdk-go/aws/session"
	_ "github.com/confluentinc/confluent-kafka-go/kafka"
	_ "github.com/confluentinc/confluent-kafka-go/v2/kafka"
	_ "github.com/elastic/go-elasticsearch/v6"
	_ "github.com/elastic/go-elasticsearch/v7"
	_ "github.com/elastic/go-elasticsearch/v8"
	_ "github.com/gin-gonic/gin"
	_ "github.com/go-chi/chi"
	_ "github.com/go-chi/chi/v5"
	_ "github.com/go-redis/redis"
	_ "github.com/go-redis/redis/v7"
	_ "github.com/go-redis/redis/v8"
	_ "github.com/gocql/gocql"
	_ "github.com/gofiber/fiber/v2"
	_ "github.com/gomodule/redigo/redis"
	_ "github.com/gorilla/mux"
	_ "github.com/graph-gophers/graphql-go"
	_ "github.com/graphql-go/graphql"
	_ "github.com/hashicorp/vault/api"
	_ "github.com/jackc/pgx/v5"
	_ "github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jinzhu/gorm"
	_ "github.com/julienschmidt/httprouter"
	_ "github.com/labstack/echo/v4"
	_ "github.com/redis/go-redis/v9"
	_ "github.com/segmentio/kafka-go"
	_ "github.com/sirupsen/logrus"
	_ "github.com/twitchtv/twirp"
	_ "go.mongodb.org/mongo-driver/mongo/options"
	_ "google.golang.org/grpc"
	_ "gorm.io/gorm"
	_ "k8s.io/client-go/rest"
)
