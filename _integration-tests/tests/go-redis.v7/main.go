// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"orchestrion/integration"
	"os"
	"runtime"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/testcontainers/testcontainers-go"
	testredis "github.com/testcontainers/testcontainers-go/modules/redis"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/go-redis/redis/v7"
)

func main() {
	if os.Getenv("DOCKER_NOT_AVAILABLE") != "" {
		log.Println("Docker is required to run this test. Exiting with status code 42!")
		os.Exit(42)
	}

	ctx := context.Background()
	server, err := testredis.RunContainer(ctx,
		testcontainers.WithImage("redis:7"),
		testcontainers.WithLogConsumers(&testcontainers.StdoutLogConsumer{}),
		testcontainers.WithHostConfigModifier(func(config *container.HostConfig) {
			if runtime.GOOS == "windows" {
				config.NetworkMode = network.NetworkNat
			}
		}),
		testcontainers.WithWaitStrategy(
			wait.ForAll(
				wait.ForLog("* Ready to accept connections"),
				wait.ForExposedPort(),
				wait.ForListeningPort("6379/tcp"),
			),
		),
	)
	if err != nil {
		log.Fatalf("Failed to start redis test container: %v\n", err)
	}
	defer server.Terminate(ctx)

	redisURI, err := server.ConnectionString(ctx)
	if err != nil {
		log.Fatalf("Failed to obtain connection string: %v\n", err)
	}
	redisURL, err := url.Parse(redisURI)
	if err != nil {
		log.Fatalf("Invalid redis connection string: %q\n", redisURI)
	}
	addr := redisURL.Host
	client := redis.NewClient(&redis.Options{Addr: addr})
	defer client.Close()

	if err := client.Set("test_key", "test_value", 0).Err(); err != nil {
		log.Fatalf("Failed to insert test data: %v", err)
	}

	mux := &http.ServeMux{}
	s := &http.Server{
		Addr:    "127.0.0.1:8090",
		Handler: mux,
	}

	mux.HandleFunc("/quit",
		//dd:ignore
		func(w http.ResponseWriter, r *http.Request) {
			log.Println("Shutdown requested...")
			defer s.Shutdown(context.Background())
			w.Write([]byte("Goodbye\n"))
		})

	mux.HandleFunc("/",
		//dd:ignore
		func(w http.ResponseWriter, r *http.Request) {
			if res, err := client.WithContext(r.Context()).Get("test_key").Result(); err != nil {
				log.Printf("Error: %v\n", err)
				w.WriteHeader(http.StatusInternalServerError)
				fmt.Fprintf(w, "%v\n", err)
			} else {
				w.Write([]byte(res))
			}
		})

	integration.OnSignal(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		s.Shutdown(ctx)
	})

	log.Print(s.ListenAndServe())
}
