// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package code

import "github.com/dave/dst"

// SliceHasLow reports whether the current slice expression has a low bound.
func (d *dot) SliceHasLow() bool {
	expression, ok := d.context.Node().(*dst.SliceExpr)
	return ok && expression.Low != nil
}

// SliceHasHigh reports whether the current slice expression has a high bound.
func (d *dot) SliceHasHigh() bool {
	expression, ok := d.context.Node().(*dst.SliceExpr)
	return ok && expression.High != nil
}

// SliceHasMax reports whether the current slice expression has a capacity bound.
func (d *dot) SliceHasMax() bool {
	expression, ok := d.context.Node().(*dst.SliceExpr)
	return ok && expression.Max != nil
}
