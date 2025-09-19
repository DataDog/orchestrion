// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package advice

import (
	"sort"
)

const (
	// DefaultNamespace is used when no namespace is specified
	DefaultNamespace = "default"

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

// NewOrderedAdvices creates a new OrderedAdvice with default values
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
func Sort(orderedAdvices []*OrderedAdvice) {
	sort.Slice(orderedAdvices, func(i, j int) bool {
		if orderedAdvices[i].namespace != orderedAdvices[j].namespace {
			return orderedAdvices[i].namespace < orderedAdvices[j].namespace
		}

		if orderedAdvices[i].order != orderedAdvices[j].order {
			return orderedAdvices[i].order < orderedAdvices[j].order
		}

		return orderedAdvices[i].Index < orderedAdvices[j].Index
	})
}
