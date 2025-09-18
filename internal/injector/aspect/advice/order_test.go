// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package advice

import (
	"testing"

	"github.com/DataDog/orchestrion/internal/fingerprint"
	"github.com/DataDog/orchestrion/internal/injector/aspect/context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockOrderableAdvice struct {
	order     int
	namespace string
	id        string
}

func (m *mockOrderableAdvice) Order() int                              { return m.order }
func (m *mockOrderableAdvice) Namespace() string                       { return m.namespace }
func (*mockOrderableAdvice) AddedImports() []string                    { return nil }
func (*mockOrderableAdvice) Apply(context.AdviceContext) (bool, error) { return false, nil }
func (*mockOrderableAdvice) Hash(*fingerprint.Hasher) error            { return nil }

type nonOrderableAdvice struct {
	id string
}

func (*nonOrderableAdvice) AddedImports() []string                    { return nil }
func (*nonOrderableAdvice) Apply(context.AdviceContext) (bool, error) { return false, nil }
func (*nonOrderableAdvice) Hash(*fingerprint.Hasher) error            { return nil }

func TestSort_BasicSorting(t *testing.T) {
	advices := []*OrderedAdvice{
		NewOrderedAdvice("aspect1", &mockOrderableAdvice{id: "z-ns", namespace: "z-namespace", order: 10}, 0),
		NewOrderedAdvice("aspect2", &mockOrderableAdvice{id: "a-ns", namespace: "a-namespace", order: 10}, 1),
	}

	Sort(advices)

	require.Len(t, advices, 2)
	assert.Equal(t, "a-ns", advices[0].Advice.(*mockOrderableAdvice).id)
	assert.Equal(t, "z-ns", advices[1].Advice.(*mockOrderableAdvice).id)

	advices = []*OrderedAdvice{
		NewOrderedAdvice("aspect1", &mockOrderableAdvice{id: "order-20", namespace: "same", order: 20}, 0),
		NewOrderedAdvice("aspect2", &mockOrderableAdvice{id: "order-10", namespace: "same", order: 10}, 1),
	}

	Sort(advices)

	require.Len(t, advices, 2)
	assert.Equal(t, "order-10", advices[0].Advice.(*mockOrderableAdvice).id)
	assert.Equal(t, "order-20", advices[1].Advice.(*mockOrderableAdvice).id)
}

func TestSort_StableSort(t *testing.T) {
	advices := []*OrderedAdvice{
		NewOrderedAdvice("aspect1", &mockOrderableAdvice{id: "first", namespace: "same", order: 10}, 0),
		NewOrderedAdvice("aspect2", &mockOrderableAdvice{id: "second", namespace: "same", order: 10}, 1),
		NewOrderedAdvice("aspect3", &mockOrderableAdvice{id: "third", namespace: "same", order: 10}, 2),
	}

	Sort(advices)

	require.Len(t, advices, 3)
	assert.Equal(t, "first", advices[0].Advice.(*mockOrderableAdvice).id)
	assert.Equal(t, "second", advices[1].Advice.(*mockOrderableAdvice).id)
	assert.Equal(t, "third", advices[2].Advice.(*mockOrderableAdvice).id)
}

func TestSort_EdgeCases(t *testing.T) {
	var advices []*OrderedAdvice
	Sort(advices)
	assert.Empty(t, advices)

	advices = []*OrderedAdvice{
		NewOrderedAdvice("aspect1", &mockOrderableAdvice{id: "only", namespace: "test", order: 10}, 0),
	}

	Sort(advices)

	require.Len(t, advices, 1)
	assert.Equal(t, "only", advices[0].Advice.(*mockOrderableAdvice).id)
}

func TestSort_NonOrderableAdvice(t *testing.T) {
	nonOrderable := &nonOrderableAdvice{id: "non-orderable"}
	orderableAdvice := &mockOrderableAdvice{id: "orderable", namespace: "z-custom", order: 10}

	orderedAdvices := []*OrderedAdvice{
		NewOrderedAdvice("aspect1", nonOrderable, 0),
		NewOrderedAdvice("aspect2", orderableAdvice, 1),
	}

	Sort(orderedAdvices)

	require.Len(t, orderedAdvices, 2)
	// Non-orderable gets defaults: namespace="default", which comes before "z-custom"
	assert.Equal(t, nonOrderable, orderedAdvices[0].Advice)
	assert.Equal(t, "orderable", orderedAdvices[1].Advice.(*mockOrderableAdvice).id)
}

func TestSort_ComplexScenario(t *testing.T) {
	// Test all sorting dimensions together:
	// - Multiple namespaces (auth, default, metrics, tracing)
	// - Multiple orders within namespaces
	// - Stable sorting for same namespace+order
	// - Mix of orderable and non-orderable advice
	advices := []*OrderedAdvice{
		// Non-orderable (gets default namespace="default", order=0)
		NewOrderedAdvice("aspect1", &nonOrderableAdvice{id: "non-orderable"}, 0),

		// Tracing namespace, order 20
		NewOrderedAdvice("aspect2", &mockOrderableAdvice{id: "tracing-20", namespace: "tracing", order: 20}, 1),

		// Auth namespace, order 5
		NewOrderedAdvice("aspect3", &mockOrderableAdvice{id: "auth-5", namespace: "auth", order: 5}, 2),

		// Default namespace, order 15 (higher than non-orderable's 0)
		NewOrderedAdvice("aspect4", &mockOrderableAdvice{id: "default-15", namespace: "default", order: 15}, 3),

		// Metrics namespace, order 10
		NewOrderedAdvice("aspect5", &mockOrderableAdvice{id: "metrics-10", namespace: "metrics", order: 10}, 4),

		// Another auth namespace, same order 5 (should come after due to stable sort)
		NewOrderedAdvice("aspect6", &mockOrderableAdvice{id: "auth-5-second", namespace: "auth", order: 5}, 5),

		// Default namespace, order 0 (same as non-orderable, should come after due to stable sort)
		NewOrderedAdvice("aspect7", &mockOrderableAdvice{id: "default-0", namespace: "default", order: 0}, 6),
	}

	Sort(advices)

	require.Len(t, advices, 7)

	// Expected order:
	// 1. auth namespace: auth-5 (index 2), auth-5-second (index 5)
	// 2. default namespace: non-orderable (index 0, order=0), default-0 (index 6, order=0), default-15 (index 3, order=15)
	// 3. metrics namespace: metrics-10 (index 4)
	// 4. tracing namespace: tracing-20 (index 1)

	assert.Equal(t, "auth-5", advices[0].Advice.(*mockOrderableAdvice).id)
	assert.Equal(t, "auth-5-second", advices[1].Advice.(*mockOrderableAdvice).id)
	assert.Equal(t, "non-orderable", advices[2].Advice.(*nonOrderableAdvice).id)
	assert.Equal(t, "default-0", advices[3].Advice.(*mockOrderableAdvice).id)
	assert.Equal(t, "default-15", advices[4].Advice.(*mockOrderableAdvice).id)
	assert.Equal(t, "metrics-10", advices[5].Advice.(*mockOrderableAdvice).id)
	assert.Equal(t, "tracing-20", advices[6].Advice.(*mockOrderableAdvice).id)
}
