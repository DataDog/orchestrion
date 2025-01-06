// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package common

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/DataDog/orchestrion/internal/log"
	"github.com/nats-io/nats.go"
)

type (
	ResponseTo[Request any] interface {
		IsResponseTo(Request)
	}
	// RequestHandler is a function that processes a request of a given type, and returns a response or an error to be
	// sent back to the client.
	RequestHandler[Request any, Response ResponseTo[Request]] func(Request) (Response, error)
)

// HandleRequest returns a NATS subscription target that calls the provided request handler in a new goroutine if the
// NATS message payload can be parsed into the specified request type, and responds to the client appropriately.
func HandleRequest[Request any, Response ResponseTo[Request]](handler RequestHandler[Request, Response]) func(*nats.Msg) {
	return func(msg *nats.Msg) {
		var req Request
		if err := json.Unmarshal(msg.Data, &req); err != nil {
			respond(msg, errorResponse{Error: err.Error()})
			return
		}

		// Spawn the handler in a new goroutine to avoid blocking the NATS subscription poller.
		go func() {
			resp, err := handler(req)
			if err != nil {
				respond(msg, errorResponse{Error: err.Error()})
				return
			}
			respond(msg, successResponse[Response]{Result: resp})
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

func respond(msg *nats.Msg, val natsResponse) {
	data, err := json.Marshal(val)
	if err != nil {
		log.Errorf("[JOBSERVER] Failed to marshal response of type %T: %v\n", val, err)
		return
	}
	if err := msg.Respond(data); err != nil {
		log.Errorf("[JOBSERVER] Failed to send response: %v\n%s\n", err, data)
		data := fmt.Sprintf(`{"error": %q}`, err.Error()) // TODO: Truncate if too long?
		if err := msg.Respond([]byte(data)); err != nil {
			log.Errorf("[JOBSERVER] Faild to send error message: %v\n", err)
		}
		return
	}
}

// UnmarshalResponse parses a response received from the job server, either into a result of the specified type, or as
// an error; depending on the response's structure.
func UnmarshalResponse[T any](data []byte) (T, error) {
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
