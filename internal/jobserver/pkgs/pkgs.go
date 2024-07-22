// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package pkgs

import (
	"github.com/datadog/orchestrion/internal/jobserver/common"
	"github.com/nats-io/nats.go"
)

const (
	subjectPrefix = "packages."

	resolveSubject = subjectPrefix + "resolve"
)

type service struct {
	resolved  common.Cache[ResolveResponse]
	serverURL string
}

func Subscribe(serverURL string, conn *nats.Conn, stats *common.CacheStats) {
	s := &service{
		resolved:  common.NewCache[ResolveResponse](stats),
		serverURL: serverURL,
	}

	conn.Subscribe(resolveSubject, common.Fork(s.resolve))
}
