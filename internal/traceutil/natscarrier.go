// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package traceutil

import (
	"strings"

	"github.com/DataDog/dd-trace-go/v2/ddtrace/tracer"
	"github.com/nats-io/nats.go"
)

type NATSCarrier struct {
	*nats.Msg
}

var _ tracer.TextMapReader = (*NATSCarrier)(nil)
var _ tracer.TextMapWriter = (*NATSCarrier)(nil)

func (c NATSCarrier) Set(key string, value string) {
	c.Msg.Header.Add(key, value)
}

func (c NATSCarrier) ForeachKey(handler func(key string, val string) error) error {
	for key, val := range c.Msg.Header {
		if err := handler(key, strings.Join(val, " ")); err != nil {
			return err
		}
	}
	return nil
}
