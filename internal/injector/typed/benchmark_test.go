// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package typed

import (
	"fmt"
	"testing"
)

// oldNewNamedType is the original regex-based implementation for benchmarking
func oldNewNamedType(n string) (nt NamedType, err error) {
	matches := typeNameRe.FindStringSubmatch(n)
	if matches == nil {
		err = fmt.Errorf("invalid NamedType syntax: %q", n)
		return nt, err
	}

	// The original implementation didn't handle pointers
	if matches[1] == "*" {
		err = fmt.Errorf("pointer types not supported: %q", n)
		return nt, err
	}

	nt.ImportPath = matches[2]
	nt.Name = matches[3]
	return nt, nil
}

// Benchmark test cases
var benchmarkCases = []struct {
	name  string
	input string
}{
	{"Simple", "string"},
	{"Qualified", "net/http.Request"},
	{"LongPath", "github.com/DataDog/orchestrion/internal/injector.Type"},
	{"WithDashes", "github.com/user-name/repo-name.Type"},
	{"PackageDot", "time.Duration"},
	{"Pointer", "*string"},
	{"PointerQualified", "*net/http.Request"},
	{"Slice", "[]string"},
	{"SlicePointer", "[]*string"},
	{"Array", "[10]string"},
	{"Map", "map[string]int"},
	{"ComplexNested", "map[string][]*net/http.Request"},
	{"VeryComplex", "*map[string][]map[int]*[]net/http.Request"},
}

// BenchmarkOldRegexParser benchmarks the old regex-based approach
func BenchmarkOldRegexParser(b *testing.B) {
	for _, tc := range benchmarkCases {
		b.Run(tc.name, func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, _ = oldNewNamedType(tc.input)
			}
		})
	}
}

// BenchmarkNewRecursiveParser benchmarks the new recursive descent parser
func BenchmarkNewRecursiveParser(b *testing.B) {
	for _, tc := range benchmarkCases {
		b.Run(tc.name, func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, _ = NewType(tc.input)
			}
		})
	}
}

// BenchmarkNewNamedTypeHelper benchmarks the NewNamedType helper that extracts NamedType
func BenchmarkNewNamedTypeHelper(b *testing.B) {
	for _, tc := range benchmarkCases {
		b.Run(tc.name, func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, _ = NewNamedType(tc.input)
			}
		})
	}
}

// Comparison benchmarks for specific operations

// BenchmarkParseSimpleTypes compares parsing performance for simple types
func BenchmarkParseSimpleTypes(b *testing.B) {
	simpleTypes := []string{"string", "int", "bool", "error", "any", "byte"}

	b.Run("OldRegex", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			for _, t := range simpleTypes {
				_, _ = oldNewNamedType(t)
			}
		}
	})

	b.Run("NewParser", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			for _, t := range simpleTypes {
				_, _ = NewType(t)
			}
		}
	})
}

// BenchmarkParseQualifiedTypes compares parsing performance for qualified types
func BenchmarkParseQualifiedTypes(b *testing.B) {
	qualifiedTypes := []string{
		"net/http.Request",
		"context.Context",
		"github.com/user/repo.Type",
		"github.com/DataDog/orchestrion/internal/injector.Type",
	}

	b.Run("OldRegex", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			for _, t := range qualifiedTypes {
				_, _ = oldNewNamedType(t)
			}
		}
	})

	b.Run("NewParser", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			for _, t := range qualifiedTypes {
				_, _ = NewType(t)
			}
		}
	})
}

// BenchmarkParseComplexTypes benchmarks parsing of complex nested types
// Note: The old parser doesn't support these, so we only benchmark the new parser
func BenchmarkParseComplexTypes(b *testing.B) {
	complexTypes := []string{
		"[][]string",
		"map[string][]int",
		"[]*map[string]interface{}",
		"map[string]map[int][]*User",
		"*map[string][]map[int]*[]net/http.Request",
	}

	b.Run("NewParser", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			for _, t := range complexTypes {
				_, _ = NewType(t)
			}
		}
	})
}

// BenchmarkParseInvalidTypes benchmarks error handling performance
func BenchmarkParseInvalidTypes(b *testing.B) {
	invalidTypes := []string{
		"",
		"123invalid",
		"map[",
		"[]",
		"[abc]string",
		"* *string",
	}

	b.Run("OldRegex", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			for _, t := range invalidTypes {
				_, _ = oldNewNamedType(t)
			}
		}
	})

	b.Run("NewParser", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			for _, t := range invalidTypes {
				_, _ = NewType(t)
			}
		}
	})
}

// Memory allocation benchmarks

// BenchmarkAllocations compares memory allocations
func BenchmarkAllocations(b *testing.B) {
	testCases := []struct {
		name  string
		input string
	}{
		{"Simple", "string"},
		{"Qualified", "net/http.Request"},
		{"Slice", "[]string"},
		{"Map", "map[string]int"},
	}

	for _, tc := range testCases {
		b.Run("OldRegex/"+tc.name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, _ = oldNewNamedType(tc.input)
			}
		})

		b.Run("NewParser/"+tc.name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, _ = NewType(tc.input)
			}
		})
	}
}
