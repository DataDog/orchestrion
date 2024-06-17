// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"context"
	"log"
	"net/http"
	"net/url"
	"strings"
)

func shortHandsWithContext(context.Context) {
	resp, err := http.Get("http://localhost:8080")
	if err != nil {
		log.Fatal(err)
	}
	resp.Body.Close()

	resp, err = http.Head("http://localhost:8080")
	if err != nil {
		log.Fatal(err)
	}
	resp.Body.Close()

	resp, err = http.Post("http://localhost:8080", "text/plain", strings.NewReader("Body"))
	if err != nil {
		log.Fatal(err)
	}
	resp.Body.Close()

	resp, err = http.PostForm("http://localhost:8080", url.Values{"key": {"value"}})
	if err != nil {
		log.Fatal(err)
	}
	resp.Body.Close()
}

func shortHandsWithRequest(_ *http.Request /* for context */) {
	resp, err := http.Get("http://localhost:8080")
	if err != nil {
		log.Fatal(err)
	}
	resp.Body.Close()

	resp, err = http.Head("http://localhost:8080")
	if err != nil {
		log.Fatal(err)
	}
	resp.Body.Close()

	resp, err = http.Post("http://localhost:8080", "text/plain", strings.NewReader("Body"))
	if err != nil {
		log.Fatal(err)
	}
	resp.Body.Close()

	resp, err = http.PostForm("http://localhost:8080", url.Values{"key": {"value"}})
	if err != nil {
		log.Fatal(err)
	}
	resp.Body.Close()
}
