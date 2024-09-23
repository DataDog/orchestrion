// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package agent

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/docker/go-connections/nat"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"

	testcontainersutils "orchestrion/integration/utils/testcontainers"
	"orchestrion/integration/validator/trace"
)

type MockAgent struct {
	host string
	port int
}

type Session struct {
	agent *MockAgent
	token uuid.UUID
}

func New(t *testing.T) *MockAgent {
	t.Helper()
	ctx := context.Background()
	exposedPort := "8126/tcp"

	agentContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Name:         "orchestrion_integration_tests",
			Image:        "ghcr.io/datadog/dd-apm-test-agent/ddapm-test-agent:latest",
			ExposedPorts: []string{exposedPort},
			WaitingFor:   wait.ForListeningPort(nat.Port(exposedPort)),
			LogConsumerCfg: &testcontainers.LogConsumerConfig{
				Consumers: []testcontainers.LogConsumer{testcontainersutils.TestLogConsumer(t)},
			},
		},
		Started: true,
		Reuse:   true,
		Logger:  testcontainers.TestLogger(t),
	})
	testcontainersutils.AssertError(t, err)

	mappedPort, err := agentContainer.MappedPort(ctx, nat.Port(exposedPort))
	require.NoError(t, err)

	host, err := agentContainer.Host(ctx)
	require.NoError(t, err)

	return &MockAgent{
		host: host,
		port: mappedPort.Int(),
	}
}

func (a *MockAgent) Addr() string {
	return fmt.Sprintf("%s:%d", a.host, a.port)
}

func (a *MockAgent) NewSession(t *testing.T) *Session {
	t.Helper()
	token, err := uuid.NewRandom()
	require.NoError(t, err)

	session := &Session{agent: a, token: token}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("http://%s/test/session/start?test_session_token=%s", a.Addr(), session.token.String()), nil)
	require.NoError(t, err)

	for {
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			select {
			case <-ctx.Done():
				require.FailNow(t, "timeout trying to create mock agent test session")
			default:
				time.Sleep(100 * time.Millisecond)
				continue
			}
		}
		if resp.StatusCode != 200 {
			require.FailNow(t, "test agent returned non-200 status code")
		}
		break
	}

	t.Logf("Started test session with ID %s\n", session.token.String())

	tracer.Start(
		tracer.WithAgentAddr(a.Addr()),
		tracer.WithSampler(tracer.NewAllSampler()),
		tracer.WithLogStartup(false),
		tracer.WithLogger(testLogger{t}),
		tracer.WithHTTPClient(&http.Client{
			Transport: &sessionTokenTransport{
				rt:           http.DefaultTransport,
				sessionToken: token.String(),
			},
			Timeout: 10 * time.Second,
		}),
	)
	t.Cleanup(tracer.Stop)

	return session
}

func (s *Session) Spans(t *testing.T) trace.Spans {
	t.Helper()
	tracer.Flush()
	tracer.Stop()

	t.Logf("Fetching spans from test-agent using session ID %s\n", s.token.String())
	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/test/session/traces?test_session_token=%s", s.agent.port, s.token.String()))
	require.NoError(t, err)
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var spans trace.Spans
	require.NoError(t, trace.ParseRaw(data, &spans))
	t.Logf("Received %d spans", len(spans))

	return spans
}

type sessionTokenTransport struct {
	rt           http.RoundTripper
	sessionToken string
}

func (s *sessionTokenTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Add("X-Datadog-Test-Session-Token", s.sessionToken)
	return s.rt.RoundTrip(req)
}

type testLogger struct {
	*testing.T
}

func (l testLogger) Log(msg string) {
	l.T.Log(msg)
}
