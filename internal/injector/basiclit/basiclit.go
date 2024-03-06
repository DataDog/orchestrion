// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package basiclit

import (
	"encoding/json"
	"fmt"
	"go/token"

	"github.com/dave/dst"
)

// String parses the value of a *dst.BasicLit to a regular go string.
func String(lit *dst.BasicLit) (string, error) {
	if lit.Kind != token.STRING {
		return "", fmt.Errorf("not a string literal: %s", lit.Kind.String())
	}
	if len(lit.Value) < 2 {
		return "", fmt.Errorf("malformed string literal (too short): %s", lit.Value)
	}

	switch lit.Value[0] {
	case '`':
		return lit.Value[1 : len(lit.Value)-1], nil
	case '"':
		// Note: We blindly assume that Go string literals have the same syntax as JSON here...
		var str string
		if err := json.Unmarshal([]byte(lit.Value), &str); err != nil {
			return "", fmt.Errorf("failed to parse string literal %s: %w", lit.Value, err)
		}
		return str, nil
	default:
		return "", fmt.Errorf("unknown string delimiter: %q", lit.Value[0])
	}
}
