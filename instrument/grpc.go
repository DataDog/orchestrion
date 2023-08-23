// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package instrument

import (
	"google.golang.org/grpc"

	grpctrace "gopkg.in/DataDog/dd-trace-go.v1/contrib/google.golang.org/grpc"
)

func GRPCStreamServerInterceptor() grpc.ServerOption {
	return grpc.StreamInterceptor(grpctrace.StreamServerInterceptor())
}

func GRPCUnaryServerInterceptor() grpc.ServerOption {
	return grpc.UnaryInterceptor(grpctrace.UnaryServerInterceptor())
}

func GRPCStreamClientInterceptor() grpc.DialOption {
	return grpc.WithStreamInterceptor(grpctrace.StreamClientInterceptor())
}

func GRPCUnaryClientInterceptor() grpc.DialOption {
	return grpc.WithUnaryInterceptor(grpctrace.UnaryClientInterceptor())
}
