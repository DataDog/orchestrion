// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package buildid

import (
	"context"
	"sync"

	"github.com/DataDog/orchestrion/internal/jobserver/common"
	"github.com/nats-io/nats.go"
	"github.com/rs/zerolog"
)

const (
	subjectPrefix = "buildid."

	versionSubject = subjectPrefix + "versionSuffix"
)

type service struct {
	stats           *common.CacheStats
	resolvedVersion VersionSuffixResponse
	mu              sync.Mutex
}

func Subscribe(ctx context.Context, conn *nats.Conn, stats *common.CacheStats) error {
	s := &service{stats: stats}
	ctx = zerolog.Ctx(ctx).With().Str("nats.subject", versionSubject).Logger().WithContext(ctx)
	_, err := conn.Subscribe(versionSubject, common.HandleRequest(ctx, s.versionSuffix))
	return err
}
