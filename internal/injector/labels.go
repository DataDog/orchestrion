// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package injector

import (
	"strings"

	"github.com/dave/dst"
)

const (
	ddIgnore = "//dd:ignore"
)

// ddIgnored returns true if the node is prefixed by a `//dd:ignore` directive.
func ddIgnored(node dst.Node) bool {
	for _, cmt := range node.Decorations().Start.All() {
		if cmt == ddIgnore || strings.HasPrefix(cmt, ddIgnore+" ") {
			return true
		}
	}
	return false
}
