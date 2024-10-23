// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"context"

	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/vektah/gqlparser/v2"
	"github.com/vektah/gqlparser/v2/ast"
)

func Serve99Designs() {
	schema := gqlparser.MustLoadSchema(&ast.Source{Input: `
	type Query {
		topLevel(id: String!): TopLevel!
	}

	type TopLevel {
		nested(id: String!): String!
	}
`})

	server := handler.New(&graphql.ExecutableSchemaMock{
		ExecFunc:   execFunc,
		SchemaFunc: func() *ast.Schema { return schema },
	})
	server.AddTransport(transport.POST{})
}

func execFunc(ctx context.Context) graphql.ResponseHandler {
	return graphql.OneShot(graphql.ErrorResponse(ctx, "not implemented"))
}
