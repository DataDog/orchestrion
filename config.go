// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package orchestrion

import (
	"fmt"
	"strings"
)

// Config holds the instrumentation config
type Config struct {
	// HTTPMode controls the technique used for HTTP instrumentation
	// The possible values are "wrap", "report"
	HTTPMode string
}

var defaultConf = Config{HTTPMode: "wrap"}

func (c *Config) Validate() error {
	c.HTTPMode = strings.ToLower(c.HTTPMode)
	switch c.HTTPMode {
	case "wrap", "report":
		return nil
	default:
		return fmt.Errorf("invalid httpmode %q, the supported values are wrap or report", c.HTTPMode)
	}
}
