// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package jobserver

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/datadog/orchestrion/internal/jobserver/buildid"
	"github.com/datadog/orchestrion/internal/jobserver/client"
	"github.com/datadog/orchestrion/internal/jobserver/common"
	"github.com/datadog/orchestrion/internal/jobserver/pkgs"
	"github.com/datadog/orchestrion/internal/log"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
)

const (
	loopback       = "127.0.0.1"
	serverUsername = "server" // User for the server itself
	sysUser        = "admin"  // User for system event access
	noPassword     = ""       // We don't need passwords, this is only to have access to system events, not for security.
)

type (
	Server struct {
		server     *server.Server     // The underlying NATS server
		conn       *nats.Conn         // The local server connection
		CacheStats *common.CacheStats // Cache statistics

		// Tracking connected clients for automatic shutdown on inactivity...
		clients           map[uint64]string
		shutdownTimer     *time.Timer
		clientsMu         sync.Mutex
		inactivityTimeout time.Duration
	}

	Options struct {
		// ServerName is the name associated with this server. If blank, a random
		// name is generated by the NATS library.
		ServerName string
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
func New(opts *Options) (*Server, error) {
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
		startTimeout = 5 * time.Second
	}

	// Creating the server instance
	userAccount := server.NewAccount("USERS")
	systemAccount := server.NewAccount("SYS")

	server, err := server.NewServer(&server.Options{
		ServerName: opts.ServerName,
		Host:       loopback,
		Port:       port,
		DontListen: opts.NoListener,
		Accounts:   []*server.Account{userAccount, systemAccount},
		Users: []*server.User{
			{Username: client.USERNAME, Password: client.NO_PASSWORD, Account: userAccount},
			{Username: serverUsername, Password: noPassword, Account: userAccount},
			{Username: sysUser, Password: noPassword, Account: systemAccount},
		},
		SystemAccount: systemAccount.Name,
	})
	if err != nil {
		return nil, fmt.Errorf("creating NATS server instance: %w", err)
	}

	if opts.EnableLogging {
		server.ConfigureLogger()
	}

	// Starting the server, and waiting for it to be ready
	server.Start()
	if !server.ReadyForConnections(startTimeout) {
		defer server.Shutdown()
		return nil, errors.New("timed out waiting for NATS server to become available")
	}

	log.Tracef("[JOBSERVER] NATS Server ready for connections on %q\n", server.ClientURL())

	// Obtaining the local server connection
	conn, err := nats.Connect(server.ClientURL(), nats.UserInfo(serverUsername, noPassword), nats.InProcessServer(server))
	if err != nil {
		defer server.Shutdown()
		return nil, fmt.Errorf("connecting to in-process NATS server instance: %w", err)
	}

	// Installing the handlers
	res := Server{
		server:     server,
		conn:       conn,
		CacheStats: &common.CacheStats{},
	}
	buildid.Subscribe(conn, res.CacheStats)
	pkgs.Subscribe(server.ClientURL(), conn, res.CacheStats)
	conn.Subscribe("clients", res.handleClients)

	if opts.InactivityTimeout > 0 {
		sysConn, err := nats.Connect(server.ClientURL(), nats.Name("server-local-admin"), nats.UserInfo(sysUser, noPassword), nats.InProcessServer(server))
		if err != nil {
			defer server.Shutdown()
			return nil, err
		}

		res.inactivityTimeout = opts.InactivityTimeout
		res.clients = make(map[uint64]string)
		sysConn.Subscribe(fmt.Sprintf("$SYS.ACCOUNT.%s.CONNECT", userAccount.Name), res.handleClientConnect)
		sysConn.Subscribe(fmt.Sprintf("$SYS.ACCOUNT.%s.DISCONNECT", userAccount.Name), res.handleClientDisconnect)

		// We don't have any (external) client just yet (we've not yet advertised our URL!), so we can start the inactivity
		// timer right away.
		res.startShutdownTimer()
	}

	// Ready for business
	return &res, nil
}

// Connect returns a client using the in-process connection to the server.
func (s *Server) Connect() (*client.Client, error) {
	conn, err := nats.Connect(
		s.server.ClientURL(),
		nats.Name("local-connect"),
		nats.UserInfo(client.USERNAME, client.NO_PASSWORD),
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
	return s.server.ClientURL()
}

// Shutdown initiates the shutdown of this server.
func (s *Server) Shutdown() {
	s.server.Shutdown()
}

// WaitForShutdown waits indefinitely for this server to have shut down.
func (s *Server) WaitForShutdown() {
	s.server.WaitForShutdown()
	log.Tracef("[JOBSERVER] %s\n", s.CacheStats)
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
		log.Errorf("[JOBSERVER] Failed to unmarshal client connect event: %v\n", err)
		return
	}

	if event.Client.User == serverUsername {
		// We don't count the server user (it shouldn't disconnect, ever!)
		return
	}

	s.clients[event.Client.ID] = event.Client.Name
	log.Tracef("[JOBSERVER] NATS client %d connected: %s\n", event.Client.ID, event.Client.Name)
	if s.shutdownTimer != nil {
		s.shutdownTimer.Stop()
		s.shutdownTimer = nil
	}
}

func (s *Server) handleClientDisconnect(msg *nats.Msg) {
	defer msg.Ack()

	var event server.DisconnectEventMsg
	if err := json.Unmarshal(msg.Data, &event); err != nil {
		log.Errorf("[JOBSERVER] Failed to unmarshal client disconnect event: %v\n", err)
		return
	}

	if event.Client.User == serverUsername {
		// We don't count the server user (it shouldn't disconnect, ever!)
		return
	}

	s.clientsMu.Lock()
	defer s.clientsMu.Unlock()
	delete(s.clients, event.Client.ID)
	log.Tracef("[JOBSERVER] NATS client %d disconnected: %s (reason: %s)\n", event.Client.ID, event.Client.Name, event.Reason)

	if len(s.clients) == 0 && s.shutdownTimer == nil {
		log.Tracef("[JOBSERVER] Last client disconnected, initiating shutdown timer...\n")
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
			log.Infof("No NATS client connected for %s, shutting down...\n", s.inactivityTimeout)
			s.Shutdown()
		}
	})
}
