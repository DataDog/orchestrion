// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"fmt"

	"github.com/twitchtv/twirp"
)

func twirpServerSample() {
	var serverOpts *twirp.ServerOptions

	serverOpts = &twirp.ServerOptions{}
	serverOpts = &twirp.ServerOptions{
		Hooks: &twirp.ServerHooks{
			RequestReceived:  nil,
			RequestRouted:    nil,
			ResponsePrepared: nil,
			ResponseSent:     nil,
			Error:            nil,
		},
		Interceptors:     nil,
		JSONSkipDefaults: false,
	}

	// these options are used in the twirp generated code
	fmt.Printf("serverOpts: %v\n", serverOpts)
}
