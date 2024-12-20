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
	"runtime/debug"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
)

type MockAgent struct {
	virtualEnv     string
	process        *exec.Cmd
	processCancel  context.CancelFunc
	currentSession atomic.Pointer[Session]
	port           int
}

type Session struct {
	agent *MockAgent
	token uuid.UUID
}

func New(t *testing.T) (*MockAgent, error) {
	var (
		agent MockAgent
		err   error
	)

	ddapmTestAgent, _ := exec.LookPath("ddapm-test-agent")
	if ddapmTestAgent == "" {
		t.Log("No ddapm-test-agent found in $PATH, installing into a python venv...")
		if agent.virtualEnv, err = os.MkdirTemp("", "orchestrion-integ-venv-*"); err != nil {
			return nil, err
		}
		t.Logf("Creating Python venv at %q...\n", agent.virtualEnv)
		if err = exec.Command("python3", "-m", "venv", agent.virtualEnv).Run(); err != nil {
			return nil, err
		}
		venvBin := filepath.Join(agent.virtualEnv, "bin")
		if runtime.GOOS == "windows" {
			venvBin = filepath.Join(agent.virtualEnv, "Scripts")
		}

		t.Logf("Installing requirements in venv...\n")
		_, thisFile, _, _ := runtime.Caller(0)
		thisDir := filepath.Dir(thisFile)
		if err = exec.Command(filepath.Join(venvBin, "pip"), "install", "-r", filepath.Join(thisDir, "requirements-dev.txt")).Run(); err != nil {
			return nil, err
		}

		ddapmTestAgent = filepath.Join(venvBin, "ddapm-test-agent")
	}

	if agent.port, err = getFreePort(); err != nil {
		return nil, err
	}
	t.Logf("Starting %s on port %d\n", ddapmTestAgent, agent.port)
	var ctx context.Context
	ctx, agent.processCancel = context.WithCancel(context.Background())
	agent.process = exec.CommandContext(
		ctx,
		ddapmTestAgent,
		fmt.Sprintf("--port=%d", agent.port),
	)
	agent.process.Stdout = os.Stdout
	agent.process.Stderr = os.Stderr

	if err = agent.process.Start(); err != nil {
		return nil, err
	}

	return &agent, nil
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
		resp, err := internalClient.Do(req)
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
			return nil, fmt.Errorf("test agent returned non-200 status code: HTTP %s", resp.Status)
		}
		break
	}

	t.Logf("Started test session with ID %s\n", session.token.String())
	tracer.Start(
		tracer.WithAgentAddr(fmt.Sprintf("127.0.0.1:%d", a.port)),
		tracer.WithHTTPRoundTripper(session),
		tracer.WithSampler(tracer.NewAllSampler()),
		tracer.WithLogStartup(false),
		tracer.WithLogger(testLogger{t}),
	)
	t.Cleanup(tracer.Stop)

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

	return os.RemoveAll(a.virtualEnv)
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
	resp, err := internalClient.Get(fmt.Sprintf("http://127.0.0.1:%d/test/session/traces?test_session_token=%s", s.agent.port, s.token.String()))
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

var (
	defaultTransport, _ = http.DefaultTransport.(*http.Transport)
	// A copy of the default transport, except it will be marked internal by orchestrion, so it is not traced.
	internalTransport = &http.Transport{
		Proxy:                 defaultTransport.Proxy,
		DialContext:           defaultTransport.DialContext,
		ForceAttemptHTTP2:     defaultTransport.ForceAttemptHTTP2,
		MaxIdleConns:          defaultTransport.MaxIdleConns,
		IdleConnTimeout:       defaultTransport.IdleConnTimeout,
		TLSHandshakeTimeout:   defaultTransport.TLSHandshakeTimeout,
		ExpectContinueTimeout: defaultTransport.ExpectContinueTimeout,
	}

	internalClient = http.Client{Transport: internalTransport}
)

func (s *Session) RoundTrip(req *http.Request) (*http.Response, error) {
	if !s.agent.currentSession.CompareAndSwap(s, s) {
		_, _ = fmt.Printf("WARNING: Span transport used after session has been closed!\n%s\n", debug.Stack())
	}

	req.Header.Set("X-Datadog-Test-Session-Token", s.token.String())
	return internalTransport.RoundTrip(req)
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
