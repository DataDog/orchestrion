// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package grpc

import (
	"context"
	"net"
	"orchestrion/integration/validator/trace"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/examples/helloworld/helloworld"
)

type TestCase struct {
	*grpc.Server
}

func (tc *TestCase) Setup(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:9090")
	require.NoError(t, err)

	tc.Server = grpc.NewServer()
	helloworld.RegisterGreeterServer(tc.Server, &server{})

	go func() { require.NoError(t, tc.Server.Serve(lis)) }()
}

func (tc *TestCase) Run(t *testing.T) {
	conn, err := grpc.NewClient("127.0.0.1:9090", grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer func() { require.NoError(t, conn.Close()) }()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	client := helloworld.NewGreeterClient(conn)
	resp, err := client.SayHello(ctx, &helloworld.HelloRequest{Name: "rob"})
	require.NoError(t, err)
	require.Equal(t, "Hello rob", resp.GetMessage())
}

func (tc *TestCase) Teardown(t *testing.T) {
	tc.Server.GracefulStop()
}

func (*TestCase) ExpectedTraces() trace.Spans {
	return trace.Spans{
		{
			Tags: map[string]any{
				"name":     "grpc.client",
				"service":  "grpc.client",
				"resource": "/helloworld.Greeter/SayHello",
				"type":     "rpc",
			},
			Children: trace.Spans{
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
