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
