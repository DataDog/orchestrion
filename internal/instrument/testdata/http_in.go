// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

func main() {
	s := http.NewServeMux()
	s.HandleFunc("/handle", myHandler)
}

func myHandler(w http.ResponseWriter, r *http.Request) {
	b, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error()))
		return
	}
	defer r.Body.Close()
	w.WriteHeader(http.StatusOK)
	w.Write(b)
}

func myClient() {
	client := &http.Client{
		Timeout: time.Second,
	}
	req, err := http.NewRequestWithContext(context.Background(),
		http.MethodPost, "http://localhost:8080",
		strings.NewReader(os.Args[1]))
	if err != nil {
		panic(err)
	}
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	if resp.Body == nil {
		return
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}
	fmt.Println(string(b))
}
