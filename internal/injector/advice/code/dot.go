// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package code

import (
	"fmt"

	"github.com/datadog/orchestrion/internal/injector/node"
)

// dot provides the `.` value to code templates, and is used to access various bits of
// information from the template's rendering context.
type dot struct {
	node          *node.Chain // The node in context of which the template is rendered
	hasExpression bool        // Whether an expression is available to be rendered in the template
}

func (d *dot) String() string {
	if d.hasExpression {
		return "_.Expr"
	}
	return fmt.Sprintf("/* %s */", d.node.String())
}
