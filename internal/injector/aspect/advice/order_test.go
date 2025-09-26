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
	advice := []*OrderedAdvice{
		NewOrderedAdvice("aspect1", &mockOrderableAdvice{id: "z-ns", namespace: "z-namespace", order: 10}, 0),
		NewOrderedAdvice("aspect2", &mockOrderableAdvice{id: "a-ns", namespace: "a-namespace", order: 10}, 1),
	}

	Sort(advice)

	require.Len(t, advice, 2)
	assert.Equal(t, "a-ns", advice[0].Advice.(*mockOrderableAdvice).id)
	assert.Equal(t, "z-ns", advice[1].Advice.(*mockOrderableAdvice).id)

	advice = []*OrderedAdvice{
		NewOrderedAdvice("aspect1", &mockOrderableAdvice{id: "order-20", namespace: "same", order: 20}, 0),
		NewOrderedAdvice("aspect2", &mockOrderableAdvice{id: "order-10", namespace: "same", order: 10}, 1),
	}

	Sort(advice)

	require.Len(t, advice, 2)
	assert.Equal(t, "order-10", advice[0].Advice.(*mockOrderableAdvice).id)
	assert.Equal(t, "order-20", advice[1].Advice.(*mockOrderableAdvice).id)
}

func TestSort_StableSort(t *testing.T) {
	advice := []*OrderedAdvice{
		NewOrderedAdvice("aspect1", &mockOrderableAdvice{id: "first", namespace: "same", order: 10}, 0),
		NewOrderedAdvice("aspect2", &mockOrderableAdvice{id: "second", namespace: "same", order: 10}, 1),
		NewOrderedAdvice("aspect3", &mockOrderableAdvice{id: "third", namespace: "same", order: 10}, 2),
	}

	Sort(advice)

	require.Len(t, advice, 3)
	assert.Equal(t, "first", advice[0].Advice.(*mockOrderableAdvice).id)
	assert.Equal(t, "second", advice[1].Advice.(*mockOrderableAdvice).id)
	assert.Equal(t, "third", advice[2].Advice.(*mockOrderableAdvice).id)
}

func TestSort_EdgeCases(t *testing.T) {
	var advice []*OrderedAdvice
	Sort(advice)
	assert.Empty(t, advice)

	advice = []*OrderedAdvice{
		NewOrderedAdvice("aspect1", &mockOrderableAdvice{id: "only", namespace: "test", order: 10}, 0),
	}

	Sort(advice)

	require.Len(t, advice, 1)
	assert.Equal(t, "only", advice[0].Advice.(*mockOrderableAdvice).id)
}

func TestSort_NonOrderableAdvice(t *testing.T) {
	nonOrderable := &nonOrderableAdvice{id: "non-orderable"}
	orderableAdvice := &mockOrderableAdvice{id: "orderable", namespace: "z-custom", order: 10}

	orderedAdvice := []*OrderedAdvice{
		NewOrderedAdvice("aspect1", nonOrderable, 0),
		NewOrderedAdvice("aspect2", orderableAdvice, 1),
	}

	Sort(orderedAdvice)

	require.Len(t, orderedAdvice, 2)
	// Non-orderable gets DefaultNamespace which comes after "z-custom"
	assert.Equal(t, "orderable", orderedAdvice[0].Advice.(*mockOrderableAdvice).id)
	assert.Equal(t, nonOrderable, orderedAdvice[1].Advice)
}

func TestSort_ComplexScenario(t *testing.T) {
	// Test all sorting dimensions together:
	// - Multiple namespaces (auth, default, metrics, tracing)
	// - Multiple orders within namespaces
	// - Stable sorting for same namespace+order
	// - Mix of orderable and non-orderable advice
	advice := []*OrderedAdvice{
		// Non-orderable (gets default namespace=DefaultNamespace, order=0)
		NewOrderedAdvice("aspect1", &nonOrderableAdvice{id: "non-orderable"}, 0),

		// Tracing namespace, order 20
		NewOrderedAdvice("aspect2", &mockOrderableAdvice{id: "tracing-20", namespace: "tracing", order: 20}, 1),

		// Auth namespace, order 5
		NewOrderedAdvice("aspect3", &mockOrderableAdvice{id: "auth-5", namespace: "auth", order: 5}, 2),

		// Default namespace, order 15 (higher than non-orderable's 0)
		NewOrderedAdvice("aspect4", &mockOrderableAdvice{id: "default-15", namespace: DefaultNamespace, order: 15}, 3),

		// Metrics namespace, order 10
		NewOrderedAdvice("aspect5", &mockOrderableAdvice{id: "metrics-10", namespace: "metrics", order: 10}, 4),

		// Another auth namespace, same order 5 (should come after due to stable sort)
		NewOrderedAdvice("aspect6", &mockOrderableAdvice{id: "auth-5-second", namespace: "auth", order: 5}, 5),

		// Default namespace, order 0 (same as non-orderable, should come after due to stable sort)
		NewOrderedAdvice("aspect7", &mockOrderableAdvice{id: "default-0", namespace: DefaultNamespace, order: 0}, 6),
	}

	Sort(advice)

	require.Len(t, advice, 7)

	// Expected order:
	// 1. auth namespace: auth-5 (index 2), auth-5-second (index 5)
	// 2. metrics namespace: metrics-10 (index 4)
	// 3. tracing namespace: tracing-20 (index 1)
	// 4. DefaultNamespace (last): non-orderable (index 0, order=0), default-0 (index 6, order=0), default-15 (index 3, order=15)

	assert.Equal(t, "auth-5", advice[0].Advice.(*mockOrderableAdvice).id)
	assert.Equal(t, "auth-5-second", advice[1].Advice.(*mockOrderableAdvice).id)
	assert.Equal(t, "metrics-10", advice[2].Advice.(*mockOrderableAdvice).id)
	assert.Equal(t, "tracing-20", advice[3].Advice.(*mockOrderableAdvice).id)
	assert.Equal(t, "non-orderable", advice[4].Advice.(*nonOrderableAdvice).id)
	assert.Equal(t, "default-0", advice[5].Advice.(*mockOrderableAdvice).id)
	assert.Equal(t, "default-15", advice[6].Advice.(*mockOrderableAdvice).id)
}

func TestSort_LargeSetWithDefaultNamespace(t *testing.T) {
	// Test a large set with namespaces before, at, and after DefaultNamespace
	// This surfaces edge cases and ensures proper ordering across the full spectrum
	advice := []*OrderedAdvice{
		// "auth" namespace (before DefaultNamespace)
		NewOrderedAdvice("auth1", &mockOrderableAdvice{id: "auth-high", namespace: "auth", order: 20}, 0),
		NewOrderedAdvice("auth2", &mockOrderableAdvice{id: "auth-low", namespace: "auth", order: 5}, 1),
		NewOrderedAdvice("auth3", &mockOrderableAdvice{id: "auth-medium", namespace: "auth", order: 10}, 2),

		// DefaultNamespace (DefaultNamespace)
		NewOrderedAdvice("default1", &mockOrderableAdvice{id: "default-high", namespace: DefaultNamespace, order: 25}, 3),
		NewOrderedAdvice("default2", &nonOrderableAdvice{id: "default-non-orderable"}, 4), // Gets DefaultNamespace + DefaultOrder
		NewOrderedAdvice("default3", &mockOrderableAdvice{id: "default-zero", namespace: DefaultNamespace, order: DefaultOrder}, 5),
		NewOrderedAdvice("default4", &mockOrderableAdvice{id: "default-low", namespace: DefaultNamespace, order: 3}, 6),

		// "metrics" namespace (after DefaultNamespace)
		NewOrderedAdvice("metrics1", &mockOrderableAdvice{id: "metrics-high", namespace: "metrics", order: 30}, 7),
		NewOrderedAdvice("metrics2", &mockOrderableAdvice{id: "metrics-low", namespace: "metrics", order: 8}, 8),

		// "tracing" namespace (after "metrics")
		NewOrderedAdvice("tracing1", &mockOrderableAdvice{id: "tracing-medium", namespace: "tracing", order: 15}, 9),
		NewOrderedAdvice("tracing2", &mockOrderableAdvice{id: "tracing-high", namespace: "tracing", order: 40}, 10),

		// "aaa" namespace (before "auth" - alphabetically first)
		NewOrderedAdvice("aaa1", &mockOrderableAdvice{id: "aaa-first", namespace: "aaa", order: 50}, 11),

		// "zzz" namespace (alphabetically last)
		NewOrderedAdvice("zzz1", &mockOrderableAdvice{id: "zzz-last", namespace: "zzz", order: 1}, 12),
	}

	Sort(advice)

	require.Len(t, advice, 13)

	// Expected order: aaa → auth → metrics → tracing → zzz → DefaultNamespace (last)
	// Within each namespace: ascending by order, then by index for ties

	expectedOrder := []string{
		"aaa-first",             // aaa, order=50
		"auth-low",              // auth, order=5
		"auth-medium",           // auth, order=10
		"auth-high",             // auth, order=20
		"metrics-low",           // metrics, order=8
		"metrics-high",          // metrics, order=30
		"tracing-medium",        // tracing, order=15
		"tracing-high",          // tracing, order=40
		"zzz-last",              // zzz, order=1
		"default-non-orderable", // DefaultNamespace, order=0 (DefaultOrder), index=4
		"default-zero",          // DefaultNamespace, order=0 (DefaultOrder), index=5
		"default-low",           // DefaultNamespace, order=3
		"default-high",          // DefaultNamespace, order=25
	}

	for i, expected := range expectedOrder {
		var actual string
		if mockAdv, ok := advice[i].Advice.(*mockOrderableAdvice); ok {
			actual = mockAdv.id
		} else if nonOrderAdv, ok := advice[i].Advice.(*nonOrderableAdvice); ok {
			actual = nonOrderAdv.id
		}
		assert.Equal(t, expected, actual, "Position %d should be %s", i, expected)
	}
}

func TestAdviceSorter(t *testing.T) {
	tests := []struct {
		name     string
		makeA    func() *OrderedAdvice
		makeB    func() *OrderedAdvice
		wantSign int
	}{
		{
			name: "NamespaceAscending",
			makeA: func() *OrderedAdvice {
				return NewOrderedAdvice("aspect-auth", &mockOrderableAdvice{id: "aspect-auth", namespace: "auth", order: 10}, 0)
			},
			makeB: func() *OrderedAdvice {
				return NewOrderedAdvice("aspect-default", &mockOrderableAdvice{id: "aspect-default", namespace: "default", order: 10}, 1)
			},
			wantSign: -1,
		},
		{
			name: "NamespaceDescending",
			makeA: func() *OrderedAdvice {
				return NewOrderedAdvice("aspect-tracing", &mockOrderableAdvice{id: "aspect-tracing", namespace: "tracing", order: 10}, 2)
			},
			makeB: func() *OrderedAdvice {
				return NewOrderedAdvice("aspect-auth", &mockOrderableAdvice{id: "aspect-auth", namespace: "auth", order: 10}, 3)
			},
			wantSign: 1,
		},
		{
			name: "OrderAscending",
			makeA: func() *OrderedAdvice {
				return NewOrderedAdvice("aspect-low", &mockOrderableAdvice{id: "aspect-low", namespace: "metrics", order: 5}, 4)
			},
			makeB: func() *OrderedAdvice {
				return NewOrderedAdvice("aspect-high", &mockOrderableAdvice{id: "aspect-high", namespace: "metrics", order: 15}, 5)
			},
			wantSign: -1,
		},
		{
			name: "OrderDescending",
			makeA: func() *OrderedAdvice {
				return NewOrderedAdvice("aspect-high", &mockOrderableAdvice{id: "aspect-high", namespace: "metrics", order: 20}, 6)
			},
			makeB: func() *OrderedAdvice {
				return NewOrderedAdvice("aspect-low", &mockOrderableAdvice{id: "aspect-low", namespace: "metrics", order: 10}, 7)
			},
			wantSign: 1,
		},
		{
			name: "IndexTieBreakerAscending",
			makeA: func() *OrderedAdvice {
				return NewOrderedAdvice("aspect-first", &mockOrderableAdvice{id: "aspect-first", namespace: "default", order: 0}, 1)
			},
			makeB: func() *OrderedAdvice {
				return NewOrderedAdvice("aspect-second", &mockOrderableAdvice{id: "aspect-second", namespace: "default", order: 0}, 8)
			},
			wantSign: -1,
		},
		{
			name: "IndexTieBreakerDescending",
			makeA: func() *OrderedAdvice {
				return NewOrderedAdvice("aspect-second", &mockOrderableAdvice{id: "aspect-second", namespace: "default", order: 0}, 9)
			},
			makeB: func() *OrderedAdvice {
				return NewOrderedAdvice("aspect-first", &mockOrderableAdvice{id: "aspect-first", namespace: "default", order: 0}, 2)
			},
			wantSign: 1,
		},
		{
			name: "FullyEqual",
			makeA: func() *OrderedAdvice {
				return NewOrderedAdvice("aspect-equal-a", &mockOrderableAdvice{id: "aspect-equal-a", namespace: "default", order: 0}, 3)
			},
			makeB: func() *OrderedAdvice {
				return NewOrderedAdvice("aspect-equal-b", &mockOrderableAdvice{id: "aspect-equal-b", namespace: "default", order: 0}, 3)
			},
			wantSign: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := tt.makeA()
			b := tt.makeB()

			result := adviceSorter(a, b)
			reverse := adviceSorter(b, a)

			switch tt.wantSign {
			case -1:
				assert.Negative(t, result)
				assert.Positive(t, reverse)
			case 1:
				assert.Positive(t, result)
				assert.Negative(t, reverse)
			default:
				assert.Equal(t, 0, result)
				assert.Equal(t, 0, reverse)
			}
		})
	}
}
