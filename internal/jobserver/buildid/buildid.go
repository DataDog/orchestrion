// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package buildid

import (
	"sync"

	"github.com/datadog/orchestrion/internal/jobserver/common"
	"github.com/nats-io/nats.go"
)

const (
	subjectPrefix = "buildid."

	versionSubject = subjectPrefix + "versionSuffix"
)

type service struct {
	stats           *common.CacheStats
	resolvedVersion string
	mu              sync.Mutex
}

func Subscribe(conn *nats.Conn, stats *common.CacheStats) error {
	s := &service{stats: stats}
	_, err := conn.Subscribe(versionSubject, common.Fork(s.versionSuffix))
	return err
}
