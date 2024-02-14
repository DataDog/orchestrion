// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"log"

	"google.golang.org/grpc"
//line <generated>:1
	grpc1 "gopkg.in/DataDog/dd-trace-go.v1/contrib/google.golang.org/grpc"
)

//line samples/client/grpc.go:14
func grpcClient() {
	conn, err := grpc.Dial("localhost:50051", grpc.WithInsecure(),
//line <generated>:1
		grpc.WithStreamInterceptor(grpc1.StreamClientInterceptor()), grpc.WithUnaryInterceptor(grpc1.UnaryClientInterceptor()))
//line samples/client/grpc.go:16
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()
}
