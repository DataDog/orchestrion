// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package agent

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"

	pb "github.com/DataDog/datadog-agent/pkg/proto/pbgo/trace"
	"github.com/stretchr/testify/require"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"

	"orchestrion/integration/validator/trace"
)

type MockAgent struct {
	T        *testing.T
	mu       sync.RWMutex
	payloads []pb.Traces
	srv      *httptest.Server
}

func (m *MockAgent) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	m.T.Logf("mockagent: handling request: %s", req.URL.String())

	switch req.URL.Path {
	case "/v0.4/traces":
		m.handleTraces(req)
	default:
		m.T.Logf("mockagent: handler not implemented for path: %s", req.URL.String())
	}

	w.WriteHeader(200)
	w.Write([]byte("{}"))
}

func (m *MockAgent) handleTraces(req *http.Request) {
	var payload pb.Traces
	err := decodeRequest(req, &payload)
	require.NoError(m.T, err)

	m.mu.Lock()
	defer m.mu.Unlock()
	m.payloads = append(m.payloads, payload)
}

func decodeRequest(req *http.Request, dest *pb.Traces) error {
	b, err := io.ReadAll(req.Body)
	if err != nil {
		return err
	}
	defer req.Body.Close()

	_, err = dest.UnmarshalMsg(b)
	return err
}

func New(t *testing.T) *MockAgent {
	return &MockAgent{T: t}
}

func (m *MockAgent) Start(t *testing.T) {
	m.T.Log("mockagent: starting")

	//dd:ignore
	srv := httptest.NewServer(m)
	m.srv = srv
	t.Cleanup(srv.Close)

	srvURL, err := url.Parse(srv.URL)
	require.NoError(t, err)

	tracer.Start(
		tracer.WithAgentAddr(srvURL.Host),
		tracer.WithSampler(tracer.NewAllSampler()),
		tracer.WithLogStartup(false),
		tracer.WithLogger(testLogger{t}),
	)
	t.Cleanup(tracer.Stop)
}

func (m *MockAgent) Spans() trace.Spans {
	m.T.Log("mockagent: fetching spans")

	tracer.Flush()
	tracer.Stop()
	m.srv.Close()

	m.mu.RLock()
	defer m.mu.RUnlock()

	spansByID := map[trace.SpanID]*trace.Span{}
	for _, payload := range m.payloads {
		for _, spans := range payload {
			for _, span := range spans {
				spansByID[trace.SpanID(span.SpanID)] = &trace.Span{
					ID:       trace.SpanID(span.SpanID),
					ParentID: trace.SpanID(span.ParentID),
					Meta:     span.Meta,
					Tags: map[string]any{
						"name":     span.Name,
						"type":     span.Type,
						"service":  span.Service,
						"resource": span.Resource,
					},
					Children: nil,
				}
			}
		}
	}
	var result trace.Spans
	for _, span := range spansByID {
		if span.ParentID == 0 {
			result = append(result, span)
			continue
		}
		parent, ok := spansByID[span.ParentID]
		if ok {
			parent.Children = append(parent.Children, span)
		}
	}
	m.T.Logf("Received %d spans", len(result))
	return result
}

type testLogger struct {
	*testing.T
}

func (l testLogger) Log(msg string) {
	l.T.Log(msg)
}
