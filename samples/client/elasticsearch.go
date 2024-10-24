// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"fmt"
	"net/http"

	esv6 "github.com/elastic/go-elasticsearch/v6"
	esv7 "github.com/elastic/go-elasticsearch/v7"
	esv8 "github.com/elastic/go-elasticsearch/v8"
)

func SampleGoElasticsearch() {
	var (
		v6Client      *esv6.Client
		v7Client      *esv7.Client
		v8Client      *esv8.Client
		v8TypedClient *esv8.TypedClient
		err           error
	)

	v6Client, err = esv6.NewDefaultClient()
	v6Client, err = esv6.NewClient(esv6.Config{})
	v6Client, err = esv6.NewClient(esv6.Config{
		Transport: http.DefaultTransport,
	})

	v7Client, err = esv7.NewDefaultClient()
	v7Client, err = esv7.NewClient(esv7.Config{})
	v7Client, err = esv7.NewClient(esv7.Config{
		Transport: http.DefaultTransport,
	})

	v8Client, err = esv8.NewDefaultClient()
	v8Client, err = esv8.NewClient(esv8.Config{})
	v8Client, err = esv8.NewClient(esv8.Config{
		Transport: http.DefaultTransport,
	})
	v8TypedClient, err = esv8.NewTypedClient(esv8.Config{})
	v8TypedClient, err = esv8.NewTypedClient(esv8.Config{
		Transport: http.DefaultTransport,
	})

	cfgPtr := &esv8.Config{
		Transport: http.DefaultTransport,
	}
	v8TypedClient, err = esv8.NewTypedClient(*cfgPtr)

	fmt.Printf("v6: %v, v7: %v, v8: %v, v8 (typed): %v, err: %v\n", v6Client, v7Client, v8Client, v8TypedClient, err)
}
