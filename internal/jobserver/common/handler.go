// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package common

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/DataDog/dd-trace-go/v2/ddtrace/ext"
	"github.com/DataDog/dd-trace-go/v2/ddtrace/tracer"
	"github.com/DataDog/orchestrion/internal/traceutil"
	"github.com/nats-io/nats.go"
	"github.com/rs/zerolog"
)

type (
	Request[Res any] interface {
		ResponseIs(Res)
		ForeachSpanTag(func(key string, val any))
	}
	// RequestHandler is a function that processes a request of a given type, and returns a response or an error to be
	// sent back to the client.
	RequestHandler[Res any, Req Request[Res]] func(context.Context, Req) (Res, error)
)

// HandleRequest returns a NATS subscription target that calls the provided request handler in a new goroutine if the
// NATS message payload can be parsed into the specified request type, and responds to the client appropriately.
func HandleRequest[Res any, Req Request[Res]](ctx context.Context, handler RequestHandler[Res, Req]) func(*nats.Msg) {
	return func(msg *nats.Msg) {
		var req Req
		if err := json.Unmarshal(msg.Data, &req); err != nil {
			respond(ctx, msg, errorResponse{Error: err.Error()})
			return
		}

		// Spawn the handler in a new goroutine to avoid blocking the NATS subscription poller.
		go func() {
			if spanCtx, err := tracer.Extract(traceutil.NATSCarrier{Msg: msg}); err == nil && spanCtx != nil {
				span := tracer.StartSpan("nats.server",
					tracer.ServiceName("orchestrion-jobserver"),
					tracer.ResourceName(msg.Subject),
					tracer.Tag(ext.SpanKind, ext.SpanKindServer),
					tracer.ChildOf(spanCtx),
				)
				defer span.Finish()
				ctx = tracer.ContextWithSpan(ctx, span)
			}

			resp, err := handler(ctx, req)
			if err != nil {
				respond(ctx, msg, errorResponse{Error: err.Error()})
				return
			}
			respond(ctx, msg, successResponse[Res]{Result: resp})
		}()
	}
}

type (
	errorResponse struct {
		Error string `json:"error"`
	}
	successResponse[T any] struct {
		Result T `json:"result"`
	}

	// natsResponse is a marker interface used to make sure the value send to respond is one of the two possible response
	// types, so that it is guaranteed that the UnmarshalResponse function can accept it.
	natsResponse interface {
		isNatsResponse()
	}
)

func (errorResponse) isNatsResponse()      {}
func (successResponse[T]) isNatsResponse() {}

func respond(ctx context.Context, msg *nats.Msg, val natsResponse) {
	log := zerolog.Ctx(ctx)

	data, err := json.Marshal(val)
	if err != nil {
		log.Error().Err(err).Type("type", val).Msg("Failed to marshal job server response")
		return
	}
	if err := msg.Respond(data); err != nil {
		log.Error().Err(err).Msg("Failed to send job server response")
		data, err := json.Marshal(errorResponse{Error: fmt.Sprintf("internal error: %v", err)})
		if err != nil {
			log.Error().Err(err).Msg("Failed to marshal job server internal error response")
			return
		}
		if err := msg.Respond(data); err != nil {
			log.Error().Err(err).Msg("Failed to send job server internal error response")
		}
		return
	}
}

// UnmarshalResponse parses a response received from the job server, either into a result of the specified type, or as
// an error; depending on the response's structure.
func UnmarshalResponse[T any](ctx context.Context, data []byte) (_ T, err error) {
	span, _ := tracer.StartSpanFromContext(ctx, "nats.UnmarshalResponse")
	defer span.Finish()

	var parsed struct {
		successResponse[T]
		errorResponse
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return parsed.Result, err
	}

	if parsed.Error != "" {
		return parsed.Result, errors.New(parsed.Error)
	}

	return parsed.Result, nil
}
