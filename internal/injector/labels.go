// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package injector

import (
	"context"
	"strings"
	"sync"

	"github.com/dave/dst"
	"github.com/rs/zerolog"
)

const (
	ddIgnore          = "//dd:ignore"
	orchestrionIgnore = "//orchestrion:ignore"
)

var warnOnce sync.Once

// isIgnored returns true if the node is prefixed by an `//orchestrion:ignore` (or the legacy `//dd:ignore`) directive.
func isIgnored(ctx context.Context, node dst.Node) bool {
	for _, cmt := range node.Decorations().Start.All() {
		if cmt == orchestrionIgnore || strings.HasPrefix(cmt, orchestrionIgnore+" ") {
			return true
		}
		if cmt == ddIgnore || strings.HasPrefix(cmt, ddIgnore+" ") {
			warnOnce.Do(func() {
				log := zerolog.Ctx(ctx)
				log.Warn().Msg("The " + ddIgnore + " directive is deprecated and may be removed in a future release of orchestrion. Please use " + orchestrionIgnore + " instead.")
			})
			return true
		}
	}
	return false
}
