// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

//go:generate go run ./generator -o dot_ast.proxies.go
package code

// AST returns the raw AST node that `.` represents in the template.
func (d *dot) AST() any {
	return newProxy[any](d.node.Node, &d.placeholders)
}
