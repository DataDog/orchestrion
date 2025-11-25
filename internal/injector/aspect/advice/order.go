// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package advice

import (
	"cmp"
	"slices"
	"strings"
)

const (
	// DefaultNamespace is used when no namespace is specified.
	// Uses the highest Unicode code point to ensure default advice
	// always executes last, giving users full control over execution
	// order by choosing any explicit namespace.
	DefaultNamespace = "\U0010ffff"

	// DefaultOrder is used when no order is specified
	DefaultOrder = 0
)

// OrderedAdvice wraps an Advice with ordering information for deterministic
// execution order across aspects.
type OrderedAdvice struct {
	Advice

	AspectID string
	Index    int // Original definition order for stable sorting

	namespace string
	order     int
}

// NewOrderedAdvice creates a new OrderedAdvice with default values
func NewOrderedAdvice(aspectID string, advice Advice, index int) *OrderedAdvice {
	orderedAdvice := &OrderedAdvice{
		AspectID: aspectID,
		Advice:   advice,
		Index:    index,
	}
	if orderableAdv, ok := advice.(OrderableAdvice); ok {
		orderedAdvice.order = orderableAdv.Order()
		orderedAdvice.namespace = orderableAdv.Namespace()
	} else {
		orderedAdvice.order = DefaultOrder
		orderedAdvice.namespace = DefaultNamespace
	}
	return orderedAdvice
}

// Sort sorts advice from multiple aspects and returns them in execution order.
// It handles both orderable and non-orderable advice, providing deterministic sorting
// based on namespace, order, and original definition order.
func Sort(orderedAdvice []*OrderedAdvice) {
	slices.SortStableFunc(orderedAdvice, adviceSorter)
}

func adviceSorter(a *OrderedAdvice, b *OrderedAdvice) int {
	if n := strings.Compare(a.namespace, b.namespace); n != 0 {
		return n
	}

	if n := cmp.Compare(a.order, b.order); n != 0 {
		return n
	}

	return cmp.Compare(a.Index, b.Index)
}
