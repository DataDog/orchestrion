// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package common

import (
	"bytes"
	"encoding/gob"
	"fmt"

	"github.com/nats-io/nats.go"
)

type Response[T any] struct {
	Value T
	Error error
}

// Sends a response to the client.
func Respond[T any](msg *nats.Msg, value T, err error) {
	var encoded bytes.Buffer
	enc := gob.NewEncoder(&encoded)
	if err := enc.Encode(Response[T]{Value: value, Error: err}); err != nil {
		panic(fmt.Errorf("failed to encode response: %w", err))
	}
	msg.Respond(encoded.Bytes())
}

func UnmarshalResponse[T any](data []byte) (T, error) {
	dec := gob.NewDecoder(bytes.NewReader(data))
	var decoded Response[T]
	if err := dec.Decode(&decoded); err != nil {
		var zero T
		return zero, fmt.Errorf("failed to decode result: %w", err)
	}
	return decoded.Value, decoded.Error
}
