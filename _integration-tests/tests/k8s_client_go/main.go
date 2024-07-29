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

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"orchestrion/integration/validator/trace"
)

type TestCase struct {
	server    *httptest.Server
	serverURL *url.URL
	client    *kubernetes.Clientset
}

func (tc *TestCase) Setup(t *testing.T) {
	tc.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Hello World"))
	}))

	// internally, this function creates a rest.Config struct literal, so it should get traced by orchestrion.
	cfg, err := clientcmd.BuildConfigFromKubeconfigGetter(tc.server.URL, func() (*clientcmdapi.Config, error) {
		return clientcmdapi.NewConfig(), nil
	})
	require.NoError(t, err)

	tsURL, err := url.Parse(tc.server.URL)
	require.NoError(t, err)
	tc.serverURL = tsURL

	client, err := kubernetes.NewForConfig(cfg)
	require.NoError(t, err)
	tc.client = client
}

func (tc *TestCase) Run(t *testing.T) {
	_, err := tc.client.CoreV1().Namespaces().List(context.TODO(), metav1.ListOptions{})
	// we should get an error here since our test server handler implementation doesn't return what the k8s client expects
	require.Error(t, err)
}

func (tc *TestCase) Teardown(t *testing.T) {
	tc.server.Close()
}

func (tc *TestCase) ExpectedTraces() trace.Spans {
	httpServerSpan := &trace.Span{
		Tags: map[string]any{
			"name":     "http.request",
			"service":  "tests.test",
			"resource": "GET /api/v1/namespaces",
			"type":     "web",
		},
		Meta: map[string]any{
			"component":        "net/http",
			"span.kind":        "server",
			"http.useragent":   rest.DefaultKubernetesUserAgent(),
			"http.status_code": "200",
			"http.host":        tc.serverURL.Host,
			"http.url":         fmt.Sprintf("%s/api/v1/namespaces", tc.server.URL),
			"http.method":      "GET",
		},
	}
	httpClientSpan := &trace.Span{
		Tags: map[string]any{
			"name":     "http.request",
			"service":  "tests.test",
			"resource": "GET /api/v1/namespaces",
			"type":     "http",
		},
		Meta: map[string]any{
			"component":                "net/http",
			"span.kind":                "client",
			"network.destination.name": "127.0.0.1",
			"http.status_code":         "200",
			"http.method":              "GET",
			"http.url":                 fmt.Sprintf("%s/api/v1/namespaces", tc.server.URL),
		},
		Children: trace.Spans{httpServerSpan},
	}
	k8sClientSpan := &trace.Span{
		Tags: map[string]any{
			"name":     "http.request",
			"service":  "tests.test",
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
