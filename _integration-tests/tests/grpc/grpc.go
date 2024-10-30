// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

//go:build integration

package grpc

import (
	"context"
	"net"
	"testing"
	"time"

	"datadoghq.dev/orchestrion/_integration-tests/validator/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/examples/helloworld/helloworld"
)

type TestCase struct {
	*grpc.Server
	addr string
}

func (tc *TestCase) Setup(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	tc.addr = lis.Addr().String()

	tc.Server = grpc.NewServer()
	helloworld.RegisterGreeterServer(tc.Server, &server{})

	go func() { assert.NoError(t, tc.Server.Serve(lis)) }()
}

func (tc *TestCase) Run(t *testing.T) {
	conn, err := grpc.NewClient(tc.addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer func() { require.NoError(t, conn.Close()) }()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	client := helloworld.NewGreeterClient(conn)
	resp, err := client.SayHello(ctx, &helloworld.HelloRequest{Name: "rob"})
	require.NoError(t, err)
	require.Equal(t, "Hello rob", resp.GetMessage())
}

func (tc *TestCase) Teardown(*testing.T) {
	tc.Server.GracefulStop()
}

func (*TestCase) ExpectedTraces() trace.Traces {
	return trace.Traces{
		{
			Tags: map[string]any{
				"name":     "grpc.client",
				"service":  "grpc.client",
				"resource": "/helloworld.Greeter/SayHello",
				"type":     "rpc",
			},
			Children: trace.Traces{
				{
					Tags: map[string]any{
						"name":     "grpc.server",
						"service":  "grpc.server",
						"resource": "/helloworld.Greeter/SayHello",
						"type":     "rpc",
					},
				},
			},
		},
	}
}
