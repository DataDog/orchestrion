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

	"github.com/DataDog/dd-trace-go/v2/ddtrace/ext"
	"github.com/DataDog/dd-trace-go/v2/ddtrace/tracer"
	"github.com/DataDog/orchestrion/internal/jobserver/common"
	"github.com/DataDog/orchestrion/internal/traceutil"
	"github.com/nats-io/nats.go"
)

const (
	Username   = "orchestrion"
	NoPassword = "" // We only use account management to have access to system events, not for security.
)

type Client struct {
	conn *nats.Conn
}

// Connect creates a new client connected to the NATS server at the specified
// address.
func Connect(addr string) (*Client, error) {
	conn, err := nats.Connect(addr, nats.Name(fmt.Sprintf("orchestrion[%d]", os.Getpid())), nats.UserInfo(Username, NoPassword))
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
	request[Res any] interface {
		Subject() string
		common.Request[Res]
	}
)

func Request[Res any, Req request[Res]](ctx context.Context, client *Client, req Req) (Res, error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "nats.client",
		tracer.ResourceName(req.Subject()),
		tracer.Tag(ext.SpanKind, ext.SpanKindClient),
		tracer.Tag(ext.SpanType, "nats"),
	)
	defer span.Finish()

	req.ForeachSpanTag(span.SetTag)

	reqData, err := func() (_ []byte, err error) {
		span := span.StartChild("json.Marshal")
		defer func() { span.Finish(tracer.WithError(err)) }()

		return json.Marshal(req)
	}()
	if err != nil {
		var zero Res
		return zero, fmt.Errorf("encoding request payload: %w", err)
	}

	msg := nats.NewMsg(req.Subject())
	msg.Data = reqData
	tracer.Inject(span.Context(), traceutil.NATSCarrier{Msg: msg})

	resp, err := client.conn.RequestMsgWithContext(ctx, msg)
	if err != nil {
		var zero Res
		return zero, err
	}

	return common.UnmarshalResponse[Res](ctx, resp.Data)
}
