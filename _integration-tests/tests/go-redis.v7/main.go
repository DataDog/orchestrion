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
	"time"

	"github.com/testcontainers/testcontainers-go"
	testredis "github.com/testcontainers/testcontainers-go/modules/redis"

	"github.com/go-redis/redis/v7"
)

func main() {
	ctx := context.Background()
	server, err := testredis.RunContainer(ctx, testcontainers.WithImage("redis:7"), testcontainers.WithLogConsumers(&testcontainers.StdoutLogConsumer{}))
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

	for attempts := 50; attempts > 0; attempts-- { // Wait for up to 5 seconds at 100ms polling interval
		_, err := client.Ping().Result()
		if err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

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
