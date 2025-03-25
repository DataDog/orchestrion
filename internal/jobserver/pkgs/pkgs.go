// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package pkgs

import (
	"context"

	"github.com/DataDog/orchestrion/internal/injector/config"
	"github.com/DataDog/orchestrion/internal/jobserver/common"
	"github.com/nats-io/nats.go"
	"github.com/rs/zerolog"
	"golang.org/x/tools/go/packages"
)

const (
	subjectPrefix = "packages."

	resolveSubject = subjectPrefix + "resolve"
	loadSubject    = subjectPrefix + "load"
)

type service struct {
	resolved  common.Cache[ResolveResponse]
	loaded    common.Cache[*packages.Package]
	graph     common.Graph
	serverURL string
}

func Subscribe(ctx context.Context, serverURL string, conn *nats.Conn, stats *common.CacheStats) (config.PackageLoader, error) {
	s := &service{
		loaded:    common.NewCache[*packages.Package](stats),
		resolved:  common.NewCache[ResolveResponse](stats),
		serverURL: serverURL,
	}

	ctx = zerolog.Ctx(ctx).With().Str("nats.subject", resolveSubject).Logger().WithContext(ctx)
	_, err := conn.Subscribe(resolveSubject, common.HandleRequest(ctx, s.resolve))
	if err != nil {
		return nil, err
	}

	_, err = conn.Subscribe(loadSubject, common.HandleRequest(ctx, s.load))
	if err != nil {
		return nil, err
	}
	return s.packageLoader, nil
}
