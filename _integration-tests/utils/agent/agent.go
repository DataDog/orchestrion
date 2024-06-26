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
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/beevik/guid"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
)

type MockAgent struct {
	virtualEnv     string
	process        *exec.Cmd
	processCancel  context.CancelFunc
	currentSession atomic.Pointer[mockSession]
	port           int
}

type mockSession struct {
	agent *MockAgent
	token *guid.Guid
}

func New(t *testing.T) (agent MockAgent, err error) {
	if agent.virtualEnv, err = os.MkdirTemp("", "orchestrion-integ-venv-*"); err != nil {
		return
	}
	t.Logf("Creating Python venv at %q...\n", agent.virtualEnv)
	if err = exec.Command("python3", "-m", "venv", agent.virtualEnv).Run(); err != nil {
		return
	}
	venvBin := filepath.Join(agent.virtualEnv, "bin")
	if runtime.GOOS == "windows" {
		venvBin = filepath.Join(agent.virtualEnv, "Scripts")
	}

	t.Logf("Installing ddapm-test-agent in venv...\n")
	if err = exec.Command(filepath.Join(venvBin, "pip"), "install", "ddapm-test-agent").Run(); err != nil {
		return
	}

	if agent.port, err = getFreePort(); err != nil {
		return
	}
	t.Logf("Starting ddapm-test-agent on port %d\n", agent.port)
	var ctx context.Context
	ctx, agent.processCancel = context.WithCancel(context.Background())
	agent.process = exec.CommandContext(
		ctx,
		filepath.Join(venvBin, "ddapm-test-agent"),
		fmt.Sprintf("--port=%d", agent.port),
	)
	if err = agent.process.Start(); err != nil {
		return
	}

	return
}

func (a *MockAgent) NewSession(t *testing.T) (session *mockSession, err error) {
	session = &mockSession{agent: a, token: guid.New()}
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
		tracer.WithService("tests.test"),
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

	a.processCancel()
	if err := a.process.Wait(); err != nil {
		return err
	}

	if err := os.RemoveAll(a.virtualEnv); err != nil {
		return err
	}

	return nil
}

func (s *mockSession) Port() int {
	return s.agent.port
}

func (s *mockSession) Close(t *testing.T) ([]byte, error) {
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
