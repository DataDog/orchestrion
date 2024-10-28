// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

//go:build integration && !windows

package go_elasticsearch

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	testelasticsearch "github.com/testcontainers/testcontainers-go/modules/elasticsearch"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"

	"orchestrion/integration/utils"
	"orchestrion/integration/validator/trace"
)

type esClient interface {
	Perform(*http.Request) (*http.Response, error)
}

type base struct {
	container *testelasticsearch.ElasticsearchContainer
	client    esClient
}

func (b *base) Setup(t *testing.T, image string, newClient func(addr string, caCert []byte) (esClient, error)) {
	ctx := context.Background()

	var err error
	b.container, err = testelasticsearch.Run(ctx,
		image,
		testcontainers.WithLogger(testcontainers.TestLogger(t)),
		utils.WithTestLogConsumer(t),
	)
	utils.AssertTestContainersError(t, err)

	b.client, err = newClient(b.container.Settings.Address, b.container.Settings.CACert)
	require.NoError(t, err)
}

func (b *base) Run(t *testing.T, doRequest func(t *testing.T, client esClient, body io.Reader)) {
	ctx := context.Background()
	span, ctx := tracer.StartSpanFromContext(ctx, "test.root")
	defer span.Finish()

	data, err := json.Marshal(struct {
		Title string `json:"title"`
	}{Title: "some-title"})
	require.NoError(t, err)

	doRequest(t, b.client, bytes.NewReader(data))
}

func (b *base) Teardown(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	assert.NoError(t, b.container.Terminate(ctx))
}

func (*base) ExpectedTraces() trace.Traces {
	return trace.Traces{
		{
			Tags: map[string]any{
				"name": "test.root",
			},
			Children: trace.Traces{
				{
					Tags: map[string]any{
						"name":     "elasticsearch.query",
						"service":  "elastic.client",
						"resource": "PUT /test/_doc/?",
						"type":     "elasticsearch",
					},
					Meta: map[string]string{
						"component": "elastic/go-elasticsearch.v6",
						"span.kind": "client",
						"db.system": "elasticsearch",
					},
					Children: trace.Traces{
						{
							Tags: map[string]any{
								"name":     "http.request",
								"service":  "elastic.client",
								"resource": "PUT /test/_doc/1",
								"type":     "http",
							},
							Meta: map[string]string{
								"component": "net/http",
								"span.kind": "client",
							},
						},
					},
				},
			},
		},
	}
}
