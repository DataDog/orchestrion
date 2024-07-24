// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package common

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/nats-io/nats.go"
)

// Sends an error response to the client, formatted as a JSON object with a
// single `"error"` key containing the error message.
func RespondError(msg *nats.Msg, err error) {
	response := fmt.Sprintf(`{ "error": %q }`, err)
	msg.Respond([]byte(response))
}

// Sends a success response to the client, formatted as a JSON object with a
// single `"result"` key containing the response value.
func RespondJSON(msg *nats.Msg, value any) {
	response := struct {
		Result any `json:"result"`
	}{value}

	data, err := json.Marshal(response)
	if err != nil {
		RespondError(msg, err)
		return
	}

	msg.Respond(data)
}

func UnmarshalResponse[T any](data []byte) (T, error) {
	var parsed struct {
		Result T      `json:"result,omitempty"`
		Error  string `json:"error,omitempty"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return parsed.Result, err
	}

	if parsed.Error != "" {
		return parsed.Result, errors.New(parsed.Error)
	}

	return parsed.Result, nil
}
