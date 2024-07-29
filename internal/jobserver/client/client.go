// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package client

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/datadog/orchestrion/internal/jobserver/common"
	"github.com/nats-io/nats.go"
)

const (
	USERNAME    = "orchestrion"
	NO_PASSWORD = "" // We only use account management to have access to system events, not for security.

	HDR_CLIENT_PID  = "client-pid"  // Header containing the client PID
	HDR_CLIENT_PPID = "client-ppid" // Header containing the client parent PID
)

type Client struct {
	conn *nats.Conn
}

// Connect creates a new client connected to the NATS server at the specified
// address.
func Connect(addr string) (*Client, error) {
	conn, err := nats.Connect(addr, nats.Name(fmt.Sprintf("orchestrion[%d]", os.Getpid())), nats.UserInfo(USERNAME, NO_PASSWORD))
	if err != nil {
		return nil, err
	}
	return New(conn), nil
}

func New(conn *nats.Conn) *Client {
	return &Client{conn: conn}
}

func (c *Client) Close() {
	c.conn.Close()
}

type (
	request interface {
		Subject() string
	}
	responseTo[Req request] interface {
		IsResponseTo(Req)
	}
)

func Request[Req request, Res responseTo[Req]](ctx context.Context, client *Client, req Req) (Res, error) {
	reqData, err := json.Marshal(req)
	if err != nil {
		var zero Res
		return zero, fmt.Errorf("encoding request payload: %w", err)
	}

	resp, err := client.conn.RequestWithContext(ctx, req.Subject(), reqData)
	if err != nil {
		var zero Res
		return zero, err
	}

	return common.UnmarshalResponse[Res](resp.Data)
}
