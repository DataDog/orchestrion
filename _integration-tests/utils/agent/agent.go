// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package agent

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"orchestrion/integration/utils"

	"github.com/google/uuid"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
)

type MockAgent struct {
	container      testcontainers.Container
	currentSession atomic.Pointer[Session]
	port           int
}

type Session struct {
	agent *MockAgent
	token uuid.UUID
}

func New(t *testing.T) (*MockAgent, error) {
	ctx := context.Background()
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "ghcr.io/datadog/dd-apm-test-agent/ddapm-test-agent:latest",
			ExposedPorts: []string{"8126"},
			WaitingFor:   wait.ForHTTP("/info"),
			Env: map[string]string{
				"PORT":      "8126",
				"LOG_LEVEL": "WARNING",
			},
			LogConsumerCfg: &testcontainers.LogConsumerConfig{
				Consumers: []testcontainers.LogConsumer{utils.TestLogConsumer(t)},
			},
		},
		Started: true,
		Logger:  testcontainers.TestLogger(t),
	})
	if err != nil {
		t.Fatalf("Could not start ddapm-test-agent: %s", err)
	}

	agentHostPort, err := container.MappedPort(ctx, "8126")
	if err != nil {
		t.Fatalf("Could not get mapped port for ddapm-test-agent: %s", err)
	}

	t.Logf("Starting ddapm-test-agent on host port %d\n", agentHostPort.Int())

	return &MockAgent{
		container: container,
		port:      agentHostPort.Int(),
	}, nil
}

func (a *MockAgent) NewSession(t *testing.T) (session *Session, err error) {
	token, err := uuid.NewRandom()
	if err != nil {
		return nil, err
	}
	session = &Session{agent: a, token: token}
	if !a.currentSession.CompareAndSwap(nil, session) {
		return nil, errors.New("a test session is already in progress")
	}
	defer func() {
		if err != nil {
			a.currentSession.Store(nil)
			session = nil
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("http://127.0.0.1:%d/test/session/start?test_session_token=%s", a.port, session.token.String()), nil)
	if err != nil {
		return nil, err
	}

	for {
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			select {
			case <-ctx.Done():
				return nil, err
			default:
				time.Sleep(100 * time.Millisecond)
				continue
			}
		}
		if resp.StatusCode != 200 {
			return nil, errors.New("test agent returned non-200 status code")
		}
		break
	}

	t.Logf("Started test session with ID %s\n", session.token.String())
	tracer.Start(
		tracer.WithAgentAddr(fmt.Sprintf("127.0.0.1:%d", a.port)),
		tracer.WithSampler(tracer.NewAllSampler()),
		tracer.WithLogStartup(false),
		tracer.WithLogger(testLogger{t}),
	)

	return session, nil
}

type testLogger struct {
	*testing.T
}

func (l testLogger) Log(msg string) {
	l.T.Log(msg)
}

func (a *MockAgent) Close() error {
	if !a.currentSession.CompareAndSwap(nil, nil) {
		return errors.New("cannot close agent while a test session is in progress")
	}

	if err := a.container.Terminate(context.Background()); err != nil {
		return fmt.Errorf("could not terminate ddapm-test-agent: %s", err)
	}

	return nil
}

func (s *Session) Port() int {
	return s.agent.port
}

func (s *Session) Close(t *testing.T) ([]byte, error) {
	if !s.agent.currentSession.CompareAndSwap(s, nil) {
		return nil, errors.New("cannot close session that is not the currently active one")
	}

	tracer.Flush()
	tracer.Stop()

	t.Logf("Closing test session with ID %s\n", s.token.String())
	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/test/session/traces?test_session_token=%s", s.agent.port, s.token.String()))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func getFreePort() (int, error) {
	addr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}

	listener, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0, err
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port, nil
}
