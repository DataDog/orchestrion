// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package typed

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewType_ComprehensiveParsing(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    Type
		expectError bool
		errorMsg    string
	}{
		// Named types
		{
			name:     "simple type",
			input:    "string",
			expected: &NamedType{Name: "string"},
		},
		{
			name:     "qualified type",
			input:    "net/http.Request",
			expected: &NamedType{ImportPath: "net/http", Name: "Request"},
		},
		{
			name:     "type with dots in package",
			input:    "github.com/user/repo.Type",
			expected: &NamedType{ImportPath: "github.com/user/repo", Name: "Type"},
		},
		{
			name:     "type with v2 in package",
			input:    "github.com/user/repo/v2.Type",
			expected: &NamedType{ImportPath: "github.com/user/repo/v2", Name: "Type"},
		},
		{
			name:     "type with dashes in package",
			input:    "github.com/user-name/repo-name.Type",
			expected: &NamedType{ImportPath: "github.com/user-name/repo-name", Name: "Type"},
		},
		{
			name:     "simple package.Type (no slash)",
			input:    "time.Duration",
			expected: &NamedType{ImportPath: "time", Name: "Duration"},
		},
		{
			name:     "context.Context",
			input:    "context.Context",
			expected: &NamedType{ImportPath: "context", Name: "Context"},
		},
		{
			name:     "runtime.g",
			input:    "runtime.g",
			expected: &NamedType{ImportPath: "runtime", Name: "g"},
		},

		// Pointer types
		{
			name:     "pointer to simple type",
			input:    "*string",
			expected: &PointerType{Elem: &NamedType{Name: "string"}},
		},
		{
			name:     "pointer to qualified type",
			input:    "*net/http.Request",
			expected: &PointerType{Elem: &NamedType{ImportPath: "net/http", Name: "Request"}},
		},
		{
			name:     "pointer with spaces",
			input:    "* string",
			expected: &PointerType{Elem: &NamedType{Name: "string"}},
		},

		// Slice types
		{
			name:     "slice of simple type",
			input:    "[]string",
			expected: &SliceType{Elem: &NamedType{Name: "string"}},
		},
		{
			name:     "slice of pointer",
			input:    "[]*string",
			expected: &SliceType{Elem: &PointerType{Elem: &NamedType{Name: "string"}}},
		},
		{
			name:     "pointer to slice",
			input:    "*[]string",
			expected: &PointerType{Elem: &SliceType{Elem: &NamedType{Name: "string"}}},
		},
		{
			name:     "slice of slice",
			input:    "[][]byte",
			expected: &SliceType{Elem: &SliceType{Elem: &NamedType{Name: "byte"}}},
		},

		// Array types
		{
			name:     "array of simple type",
			input:    "[10]string",
			expected: &ArrayType{Size: 10, Elem: &NamedType{Name: "string"}},
		},
		{
			name:     "array with hex size",
			input:    "[0xFF]byte",
			expected: &ArrayType{Size: 255, Elem: &NamedType{Name: "byte"}},
		},
		{
			name:     "array with octal size",
			input:    "[077]byte",
			expected: &ArrayType{Size: 63, Elem: &NamedType{Name: "byte"}},
		},
		{
			name:     "array of arrays",
			input:    "[3][4]int",
			expected: &ArrayType{Size: 3, Elem: &ArrayType{Size: 4, Elem: &NamedType{Name: "int"}}},
		},
		{
			name:     "pointer to array",
			input:    "*[32]byte",
			expected: &PointerType{Elem: &ArrayType{Size: 32, Elem: &NamedType{Name: "byte"}}},
		},

		// Map types
		{
			name:     "simple map",
			input:    "map[string]int",
			expected: &MapType{Key: &NamedType{Name: "string"}, Value: &NamedType{Name: "int"}},
		},
		{
			name:     "map with pointer value",
			input:    "map[string]*User",
			expected: &MapType{Key: &NamedType{Name: "string"}, Value: &PointerType{Elem: &NamedType{Name: "User"}}},
		},
		{
			name:     "map with slice value",
			input:    "map[string][]byte",
			expected: &MapType{Key: &NamedType{Name: "string"}, Value: &SliceType{Elem: &NamedType{Name: "byte"}}},
		},
		{
			name:     "nested maps",
			input:    "map[string]map[int]bool",
			expected: &MapType{Key: &NamedType{Name: "string"}, Value: &MapType{Key: &NamedType{Name: "int"}, Value: &NamedType{Name: "bool"}}},
		},
		{
			name:     "pointer to map",
			input:    "*map[string]int",
			expected: &PointerType{Elem: &MapType{Key: &NamedType{Name: "string"}, Value: &NamedType{Name: "int"}}},
		},

		// Complex nested types
		{
			name:  "slice of maps of slices",
			input: "[]map[string][]int",
			expected: &SliceType{
				Elem: &MapType{
					Key:   &NamedType{Name: "string"},
					Value: &SliceType{Elem: &NamedType{Name: "int"}},
				},
			},
		},
		{
			name:  "map of slices of pointers",
			input: "map[string][]*User",
			expected: &MapType{
				Key: &NamedType{Name: "string"},
				Value: &SliceType{
					Elem: &PointerType{Elem: &NamedType{Name: "User"}},
				},
			},
		},
		{
			name:  "very complex nested type",
			input: "*map[string][]map[int]*[]net/http.Request",
			expected: &PointerType{
				Elem: &MapType{
					Key: &NamedType{Name: "string"},
					Value: &SliceType{
						Elem: &MapType{
							Key: &NamedType{Name: "int"},
							Value: &PointerType{
								Elem: &SliceType{
									Elem: &NamedType{ImportPath: "net/http", Name: "Request"},
								},
							},
						},
					},
				},
			},
		},

		// Error cases
		{
			name:        "empty string",
			input:       "",
			expectError: true,
		},
		{
			name:        "invalid identifier",
			input:       "123invalid",
			expectError: true,
		},
		{
			name:        "array with invalid size",
			input:       "[abc]string",
			expectError: true,
		},
		{
			name:        "array with negative size",
			input:       "[-5]string",
			expectError: true,
		},
		{
			name:        "array with float size",
			input:       "[3.14]string",
			expectError: true,
		},
		{
			name:        "incomplete slice",
			input:       "[]",
			expectError: true,
		},
		{
			name:        "incomplete array",
			input:       "[10]",
			expectError: true,
		},
		{
			name:        "incomplete map",
			input:       "map[string]",
			expectError: true,
		},
		{
			name:        "map without brackets",
			input:       "mapstring]int",
			expectError: true,
		},
		{
			name:        "map with missing key",
			input:       "map[]int",
			expectError: true,
		},
		{
			name:        "double pointer with space",
			input:       "* *string",
			expectError: true,
		},
		{
			name:        "channel type (not supported)",
			input:       "chan int",
			expectError: true,
		},
		{
			name:        "bidirectional channel (not supported)",
			input:       "chan<- int",
			expectError: true,
		},
		{
			name:        "function type (not supported)",
			input:       "func(int) string",
			expectError: true,
		},
		{
			name:        "interface type (not supported)",
			input:       "interface{}",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := NewType(tt.input)
			if tt.expectError {
				require.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

// TestParseTypeRegex tests the regex patterns used for parsing
func TestParseTypeRegex(t *testing.T) {
	tests := []struct {
		name     string
		pattern  string
		input    string
		expected []string
	}{
		{
			name:     "named type regex - simple",
			pattern:  typeNameRe.String(),
			input:    "string",
			expected: []string{"string", "", "", "string"},
		},
		{
			name:     "named type regex - pointer",
			pattern:  typeNameRe.String(),
			input:    "*string",
			expected: []string{"*string", "*", "", "string"},
		},
		{
			name:     "named type regex - qualified",
			pattern:  typeNameRe.String(),
			input:    "net/http.Request",
			expected: []string{"net/http.Request", "", "net/http", "Request"},
		},
		{
			name:     "named type regex - pointer to qualified",
			pattern:  typeNameRe.String(),
			input:    "*net/http.Request",
			expected: []string{"*net/http.Request", "*", "net/http", "Request"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches := typeNameRe.FindStringSubmatch(tt.input)
			assert.Equal(t, tt.expected, matches)
		})
	}
}
