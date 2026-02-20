// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package jobserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"sync"
	"time"

	"github.com/DataDog/orchestrion/internal/jobserver/buildid"
	"github.com/DataDog/orchestrion/internal/jobserver/client"
	"github.com/DataDog/orchestrion/internal/jobserver/common"
	"github.com/DataDog/orchestrion/internal/jobserver/inject"
	"github.com/DataDog/orchestrion/internal/jobserver/nbt"
	"github.com/DataDog/orchestrion/internal/jobserver/pkgs"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/rs/zerolog"
)

const (
	serverUsername = "server" // User for the server itself
	sysUser        = "admin"  // User for system event access
	noPassword     = ""       // We don't need passwords, this is only to have access to system events, not for security.
)

var (
	loopback = "127.0.0.1"
)

type (
	Server struct {
		server     *server.Server     // The underlying NATS server
		CacheStats *common.CacheStats // Cache statistics
		clientURL  string             // The client URL to use for connecting to this server
		log        zerolog.Logger

		shutdownHooks []func(context.Context) error

		// Tracking connected clients for automatic shutdown on inactivity...
		clients           map[uint64]string
		shutdownTimer     *time.Timer
		clientsMu         sync.Mutex
		inactivityTimeout time.Duration
	}

	Options struct {
		// Port is the port on which the server will listen for connections. If
		// zero, a random available port is used.
		Port int
		// StartTimeout is the maximum time to wait for the server to become ready
		// to accept connections. If zero, the default timeout of 5 seconds is used.
		StartTimeout time.Duration
		// EnableLogging enables server logging.
		EnableLogging bool
		// InactivityTimeout is the maximum time to wait for a ping from a client
		// before automatically shutting down. If zero, the server will not shut
		// down automatically.
		InactivityTimeout time.Duration
		// NoListener disables the network listener, only allowing in-process
		// connections to be made to this server instead.
		NoListener bool
	}
)

// New initializes and starts a new NATS server with the provided options. The
// server only listens on the loopback interface.
func New(ctx context.Context, opts *Options) (srv *Server, err error) {
	log := zerolog.Ctx(ctx).With().Str("process", "server").Logger()
	ctx = log.WithContext(ctx)

	if opts == nil {
		opts = &Options{}
	}

	// Computing defaults
	port := opts.Port
	if port == 0 {
		port = server.RANDOM_PORT
	}
	startTimeout := opts.StartTimeout
	if startTimeout == 0 {
		startTimeout = 10 * time.Second
	}

	// Creating the server instance
	userAccount := server.NewAccount("USERS")
	systemAccount := server.NewAccount("SYS")

	server, err := server.NewServer(&server.Options{
		ServerName: fmt.Sprintf("github.com/DataDog/orchestrion/internal/jobserver[%d]", os.Getpid()),
		Host:       getLoopback(log),
		Port:       port,
		DontListen: opts.NoListener,
		Accounts:   []*server.Account{userAccount, systemAccount},
		Users: []*server.User{
			{Username: client.Username, Password: client.NoPassword, Account: userAccount},
			{Username: serverUsername, Password: noPassword, Account: userAccount},
			{Username: sysUser, Password: noPassword, Account: systemAccount},
		},
		SystemAccount: systemAccount.Name,
		MaxPayload:    server.MAX_PAYLOAD_MAX_SIZE,
	})
	if err != nil {
		return nil, fmt.Errorf("creating NATS server instance: %w", err)
	}

	if opts.EnableLogging {
		server.ConfigureLogger()
	}

	// Starting the server, and waiting for it to be ready
	server.Start()

	defer func() {
		if srv == nil && err != nil {
			// Shut down the server immediately, as we are returning an error and no
			// server, so the caller wouldn't be able to do this by themselves.
			server.Shutdown()
		}
	}()

	if !server.ReadyForConnections(startTimeout) {
		return nil, errors.New("timed out waiting for NATS server to become available")
	}

	var clientURL string
	if opts.NoListener {
		// "Any" URL will do here, it's not actually used...
		clientURL = "nats://localhost:0"
	} else {
		// We don't use `server.ClientURL()` here because it currently returns an invalid URL is the
		// listener address is IPv6 (see: https://github.com/nats-io/nats-server/issues/5721)
		clientURL = fmt.Sprintf("nats://%s", server.Addr())
	}

	log.Trace().Str("url", clientURL).Msg("NATS Server ready for connections")

	// Obtaining the local server connection
	conn, err := nats.Connect(clientURL, nats.UserInfo(serverUsername, noPassword), nats.InProcessServer(server))
	if err != nil {
		return nil, fmt.Errorf("connecting to in-process NATS server instance: %w", err)
	}

	// Installing the handlers
	res := Server{
		server:     server,
		CacheStats: &common.CacheStats{},
		clientURL:  clientURL,
		log:        log,
	}
	pkgLoader, err := pkgs.Subscribe(ctx, clientURL, conn, res.CacheStats)
	if err != nil {
		return nil, err
	}
	if err := buildid.Subscribe(ctx, conn, pkgLoader, res.CacheStats); err != nil {
		return nil, err
	}
	if err := inject.Subscribe(ctx, conn, pkgLoader); err != nil {
		return nil, err
	}
	cleanup, err := nbt.Subscribe(ctx, conn)
	if err != nil {
		return nil, err
	}
	res.onShutdown(cleanup)
	if _, err := conn.Subscribe("clients", res.handleClients); err != nil {
		return nil, err
	}

	// Wait until all subscriptions have been processed by the server...
	if err := conn.Flush(); err != nil {
		return nil, err
	}

	if opts.InactivityTimeout > 0 {
		sysConn, err := nats.Connect(clientURL, nats.Name("server-local-admin"), nats.UserInfo(sysUser, noPassword), nats.InProcessServer(server))
		if err != nil {
			return nil, err
		}

		res.inactivityTimeout = opts.InactivityTimeout
		res.clients = make(map[uint64]string)
		if _, err := sysConn.Subscribe(fmt.Sprintf("$SYS.ACCOUNT.%s.CONNECT", userAccount.Name), res.handleClientConnect); err != nil {
			return nil, err
		}
		if _, err := sysConn.Subscribe(fmt.Sprintf("$SYS.ACCOUNT.%s.DISCONNECT", userAccount.Name), res.handleClientDisconnect); err != nil {
			return nil, err
		}

		// Wait until all subscriptions have been processed by the server...
		if err := sysConn.Flush(); err != nil {
			return nil, err
		}

		// We don't have any (external) client just yet (we've not yet advertised our URL!), so we can start the inactivity
		// timer right away.
		res.startShutdownTimer()
	}

	// Ready for business
	return &res, nil
}

func (s *Server) onShutdown(cb func(context.Context) error) {
	s.shutdownHooks = append(s.shutdownHooks, cb)
}

// Connect returns a client using the in-process connection to the server.
func (s *Server) Connect() (*client.Client, error) {
	conn, err := nats.Connect(
		s.clientURL,
		nats.Name("local-connect"),
		nats.UserInfo(client.Username, client.NoPassword),
		nats.InProcessServer(s.server),
	)
	if err != nil {
		return nil, err
	}
	return client.New(conn), nil
}

// ClientURL returns the URL connection string clients should use to connect to
// this NATS server.
func (s *Server) ClientURL() string {
	return s.clientURL
}

// Shutdown initiates the shutdown of this server.
func (s *Server) Shutdown() {
	s.server.Shutdown()
	go s.WaitForShutdown()
}

// WaitForShutdown waits indefinitely for this server to have shut down.
func (s *Server) WaitForShutdown() {
	s.server.WaitForShutdown()
	s.log.Trace().Msg(s.CacheStats.String())

	ctx := s.log.WithContext(context.Background())
	for _, cb := range s.shutdownHooks {
		if err := cb(ctx); err != nil {
			s.log.Error().Err(err).Msg("Shutdown hook error")
		}
	}
}

func (s *Server) handleClients(msg *nats.Msg) {
	s.clientsMu.Lock()
	defer s.clientsMu.Unlock()

	data, _ := json.MarshalIndent(s.clients, "", "  ")
	msg.Respond(data)
}

func (s *Server) handleClientConnect(msg *nats.Msg) {
	defer msg.Ack()

	// Acquire the lock early, so that we process the request before the automatic shutdown happens,
	// since we are most likely going to be cancelling it.
	s.clientsMu.Lock()
	defer s.clientsMu.Unlock()

	var event server.ConnectEventMsg
	if err := json.Unmarshal(msg.Data, &event); err != nil {
		s.log.Error().Err(err).Msg("Failed to unmarshal client connect event")
		return
	}

	if event.Client.User == serverUsername {
		// We don't count the server user (it shouldn't disconnect, ever!)
		return
	}

	s.clients[event.Client.ID] = event.Client.Name
	s.log.Trace().Uint64("client.id", event.Client.ID).Str("client.name", event.Client.Name).Msg("NATS client connected")
	if s.shutdownTimer != nil {
		s.shutdownTimer.Stop()
		s.shutdownTimer = nil
	}
}

func (s *Server) handleClientDisconnect(msg *nats.Msg) {
	defer msg.Ack()

	var event server.DisconnectEventMsg
	if err := json.Unmarshal(msg.Data, &event); err != nil {
		s.log.Error().Err(err).Msg("Failed to unmarshal client disconnect event")
		return
	}

	if event.Client.User == serverUsername {
		// We don't count the server user (it shouldn't disconnect, ever!)
		return
	}

	s.clientsMu.Lock()
	defer s.clientsMu.Unlock()
	delete(s.clients, event.Client.ID)
	s.log.Trace().Uint64("client.id", event.Client.ID).Str("client.name", event.Client.Name).Str("reason", event.Reason).Msg("NATS client disconnected")

	if len(s.clients) == 0 && s.shutdownTimer == nil {
		s.log.Trace().Msg("Last client disconnected, initiating shutdown timer...")
		s.startShutdownTimer()
	}
}

// startShutdownTimer initiates the automated shutdown timer. The caller must guarantee it has exclusive access to the
// underlying server instance (e.g, during initialization), or have acquired `s.clientsMu`.
func (s *Server) startShutdownTimer() {
	s.shutdownTimer = time.AfterFunc(s.inactivityTimeout, func() {
		s.clientsMu.Lock()
		defer s.clientsMu.Unlock()

		if len(s.clients) == 0 {
			s.log.Info().Dur("timeout", s.inactivityTimeout).Msg("No NATS client connected since shutdown timer started, shutting down...")
			s.Shutdown()
		}
	})
}

var getLoopbackOnce sync.Once

// Tries to identify a loopback IP address from available interfaces. This is
// done to ensure the server will work even if the host runs an IPv6-only
// stack, as we would discover `::1` appropriately.
func getLoopback(log zerolog.Logger) string {
	getLoopbackOnce.Do(func() {
		addrs, err := net.InterfaceAddrs()
		if err != nil {
			log.Warn().Err(err).Msg("Could not determine list of network interface addresses. Orchestrion requires at least one loopback interface to be available.")
			return
		}

		for _, addr := range addrs {
			if addr, ok := addr.(*net.IPNet); ok && addr.IP.IsLoopback() {
				loopback = addr.IP.String()
				return
			}
		}
	})

	return loopback
}
