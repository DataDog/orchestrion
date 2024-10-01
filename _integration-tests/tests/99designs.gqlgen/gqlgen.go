// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

//go:build integration

package gqlgen

import (
	"context"
	"encoding/json"
	"fmt"
	"orchestrion/integration/validator/trace"
	"testing"

	"github.com/99designs/gqlgen/client"
	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/stretchr/testify/require"
	"github.com/vektah/gqlparser/v2"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

type TestCase struct {
	server *handler.Server
}

func (tc *TestCase) Setup(*testing.T) {
	schema := gqlparser.MustLoadSchema(&ast.Source{Input: `
		type Query {
			topLevel(id: String!): TopLevel!
		}

		type TopLevel {
			nested(id: String!): String!
		}
	`})

	tc.server = handler.New(&graphql.ExecutableSchemaMock{
		ExecFunc:   execFunc,
		SchemaFunc: func() *ast.Schema { return schema },
	})
	tc.server.AddTransport(transport.POST{})
}

func (tc *TestCase) Run(t *testing.T) {
	c := client.New(tc.server)

	const (
		topLevelAttack = "he protec"
		nestedAttack   = "he attac, but most importantly: he Tupac"
	)

	var resp map[string]any
	require.NoError(t, c.Post(`
		query TestQuery($topLevelId: String!, $nestedId: String!) {
			topLevel(id: $topLevelId) {
				nested(id: $nestedId)
			}
		}
		`,
		&resp,
		client.Var("topLevelId", topLevelAttack),
		client.Var("nestedId", nestedAttack),
		client.Operation("TestQuery"),
	))

	require.Equal(t, map[string]any{
		"topLevel": map[string]any{
			"nested": fmt.Sprintf("%s/%s", topLevelAttack, nestedAttack),
		},
	}, resp)
}

func (*TestCase) Teardown(*testing.T) {}

func (*TestCase) ExpectedTraces() trace.Traces {
	return trace.Traces{
		{
			Tags: map[string]any{
				"name":    "graphql.query",
				"service": "graphql",
				"type":    "graphql",
			},
			Meta: map[string]string{
				"component": "99designs/gqlgen",
				"span.kind": "server",
			},
			Children: trace.Traces{
				{
					Tags: map[string]any{
						"name":     "graphql.field",
						"service":  "graphql",
						"resource": "TopLevel.nested",
					},
					Meta: map[string]string{
						"component":              "99designs/gqlgen",
						"graphql.operation.type": "query",
						"graphql.field":          "nested",
					},
				},
				{
					Tags: map[string]any{
						"name":     "graphql.read",
						"service":  "graphql",
						"resource": "graphql.read",
					},
					Meta: map[string]string{
						"component": "99designs/gqlgen",
					},
				},
				{
					Tags: map[string]any{
						"name":     "graphql.parse",
						"service":  "graphql",
						"resource": "graphql.parse",
					},
					Meta: map[string]string{
						"component": "99designs/gqlgen",
					},
				},
				{
					Tags: map[string]any{
						"name":     "graphql.validate",
						"service":  "graphql",
						"resource": "graphql.validate",
					},
					Meta: map[string]string{
						"component": "99designs/gqlgen",
					},
				},
				{
					Tags: map[string]any{
						"name":     "graphql.field",
						"service":  "graphql",
						"resource": "Query.topLevel",
					},
					Meta: map[string]string{
						"component":              "99designs/gqlgen",
						"graphql.operation.type": "query",
						"graphql.field":          "topLevel",
					},
				},
			},
		},
	}
}

func execFunc(ctx context.Context) graphql.ResponseHandler {
	type topLevel struct{ id string }

	switch op := graphql.GetOperationContext(ctx); op.Operation.Operation {
	case ast.Query:
		return func(ctx context.Context) *graphql.Response {
			fields := graphql.CollectFields(op, op.Operation.SelectionSet, []string{"Query"})
			var (
				val    = make(map[string]any, len(fields))
				errors gqlerror.List
			)

			for _, field := range fields {
				ctx := graphql.WithFieldContext(ctx, &graphql.FieldContext{Object: "Query", Field: field, Args: field.ArgumentMap(op.Variables)})
				fieldVal, err := op.ResolverMiddleware(ctx, func(context.Context) (any, error) {
					switch field.Name {
					case "topLevel":
						arg := field.Arguments.ForName("id")
						id, err := arg.Value.Value(op.Variables)
						strId, _ := id.(string)
						return &topLevel{strId}, err
					default:
						return nil, fmt.Errorf("unknown field: %q", field.Name)
					}
				})
				if err != nil {
					errors = append(errors, gqlerror.Errorf("%v", err))
					continue
				}
				redux := make(map[string]any, len(field.SelectionSet))
				for _, nested := range graphql.CollectFields(op, field.SelectionSet, []string{"TopLevel"}) {
					ctx := graphql.WithFieldContext(ctx, &graphql.FieldContext{Object: "TopLevel", Field: nested, Args: nested.ArgumentMap(op.Variables)})
					nestedVal, err := op.ResolverMiddleware(ctx, func(context.Context) (any, error) {
						switch nested.Name {
						case "nested":
							arg := nested.Arguments.ForName("id")
							topVal, _ := fieldVal.(*topLevel)
							id, err := arg.Value.Value(op.Variables)
							strId, _ := id.(string)
							return fmt.Sprintf("%s/%s", topVal.id, strId), err
						default:
							return nil, fmt.Errorf("unknown field: %q", nested.Name)
						}
					})
					if err != nil {
						errors = append(errors, gqlerror.Errorf("%v", err))
						continue
					}
					redux[nested.Alias] = nestedVal
				}
				val[field.Alias] = redux
			}

			data, err := json.Marshal(val)
			if err != nil {
				errors = append(errors, gqlerror.Errorf("%v", err))
			}
			return &graphql.Response{Data: data, Errors: errors}
		}
	default:
		return graphql.OneShot(graphql.ErrorResponse(ctx, "not implemented"))
	}
}
