// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package k8sclientgo

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"orchestrion/integration/validator/trace"
)

type base struct {
	server    *httptest.Server
	serverURL *url.URL
	client    *kubernetes.Clientset
}

func (b *base) setup(t *testing.T) {
	b.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Hello World"))
	}))
	tsURL, err := url.Parse(b.server.URL)
	require.NoError(t, err)
	b.serverURL = tsURL
}

func (b *base) teardown(_ *testing.T) {
	b.server.Close()
}

func (b *base) run(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	_, err := b.client.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})

	// we should get an error here since our test server handler implementation doesn't return what the k8s client expects
	require.EqualError(t, err, "serializer for text/plain; charset=utf-8 doesn't exist")
}

func (b *base) expectedSpans() trace.Spans {
	httpServerSpan := &trace.Span{
		Tags: map[string]any{
			"name":     "http.request",
			"resource": "GET /api/v1/namespaces",
			"type":     "web",
		},
		Meta: map[string]any{
			"component":        "net/http",
			"span.kind":        "server",
			"http.useragent":   rest.DefaultKubernetesUserAgent(),
			"http.status_code": "200",
			"http.host":        b.serverURL.Host,
			"http.url":         fmt.Sprintf("%s/api/v1/namespaces", b.server.URL),
			"http.method":      "GET",
		},
	}
	httpClientSpan := &trace.Span{
		Tags: map[string]any{
			"name":     "http.request",
			"resource": "GET /api/v1/namespaces",
			"type":     "http",
		},
		Meta: map[string]any{
			"component":                "net/http",
			"span.kind":                "client",
			"network.destination.name": "127.0.0.1",
			"http.status_code":         "200",
			"http.method":              "GET",
			"http.url":                 fmt.Sprintf("%s/api/v1/namespaces", b.server.URL),
		},
		Children: trace.Spans{httpServerSpan},
	}
	k8sClientSpan := &trace.Span{
		Tags: map[string]any{
			"name":     "http.request",
			"resource": "GET namespaces",
			"type":     "http",
		},
		Meta: map[string]any{
			"component": "k8s.io/client-go/kubernetes",
			"span.kind": "client",
		},
		Children: trace.Spans{httpClientSpan},
	}
	return trace.Spans{k8sClientSpan}
}
