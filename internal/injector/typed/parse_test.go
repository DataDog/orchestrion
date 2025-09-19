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
			name:     "array with traditional octal size (010 = 8)",
			input:    "[010]byte",
			expected: &ArrayType{Size: 8, Elem: &NamedType{Name: "byte"}},
		},
		{
			name:     "array with new octal size 0o (0o77 = 63)",
			input:    "[0o77]byte",
			expected: &ArrayType{Size: 63, Elem: &NamedType{Name: "byte"}},
		},
		{
			name:     "array with new octal size 0O (0O77 = 63)",
			input:    "[0O77]byte",
			expected: &ArrayType{Size: 63, Elem: &NamedType{Name: "byte"}},
		},
		{
			name:     "array with binary size 0b (0b1111 = 15)",
			input:    "[0b1111]byte",
			expected: &ArrayType{Size: 15, Elem: &NamedType{Name: "byte"}},
		},
		{
			name:     "array with binary size 0B (0B1010 = 10)",
			input:    "[0B1010]byte",
			expected: &ArrayType{Size: 10, Elem: &NamedType{Name: "byte"}},
		},
		{
			name:     "array with zero size",
			input:    "[0]byte",
			expected: &ArrayType{Size: 0, Elem: &NamedType{Name: "byte"}},
		},
		{
			name:     "array with large hex size",
			input:    "[0x1000]byte",
			expected: &ArrayType{Size: 4096, Elem: &NamedType{Name: "byte"}},
		},
		{
			name:     "array with all literal formats",
			input:    "[32]byte", // Common for crypto hashes
			expected: &ArrayType{Size: 32, Elem: &NamedType{Name: "byte"}},
		},
		{
			name:     "array with decimal size with underscores",
			input:    "[1_000]byte",
			expected: &ArrayType{Size: 1000, Elem: &NamedType{Name: "byte"}},
		},
		{
			name:     "array with hex size with underscores",
			input:    "[0xFF_FF]byte",
			expected: &ArrayType{Size: 65535, Elem: &NamedType{Name: "byte"}},
		},
		{
			name:     "array with binary size with underscores",
			input:    "[0b1111_0000]byte",
			expected: &ArrayType{Size: 240, Elem: &NamedType{Name: "byte"}},
		},
		{
			name:     "array with octal size with underscores",
			input:    "[0o377_777]byte",
			expected: &ArrayType{Size: 131071, Elem: &NamedType{Name: "byte"}},
		},
		{
			name:     "array with complex underscores",
			input:    "[1_2_3_4]byte",
			expected: &ArrayType{Size: 1234, Elem: &NamedType{Name: "byte"}},
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
			name:        "array with invalid hex digit",
			input:       "[0xG]string",
			expectError: true,
		},
		{
			name:        "array with empty hex",
			input:       "[0x]string",
			expectError: true,
		},
		{
			name:        "array with invalid binary digit",
			input:       "[0b2]string",
			expectError: true,
		},
		{
			name:        "array with empty binary",
			input:       "[0b]string",
			expectError: true,
		},
		{
			name:        "array with invalid octal digit (traditional)",
			input:       "[08]string",
			expectError: true,
		},
		{
			name:        "array with invalid octal digit (new format)",
			input:       "[0o8]string",
			expectError: true,
		},
		{
			name:        "array with empty octal",
			input:       "[0o]string",
			expectError: true,
		},
		{
			name:        "array with leading underscore in decimal",
			input:       "[_123]string",
			expectError: true,
		},
		{
			name:        "array with trailing underscore in decimal",
			input:       "[123_]string",
			expectError: true,
		},
		{
			name:        "array with consecutive underscores",
			input:       "[1__23]string",
			expectError: true,
		},
		{
			name:        "array with underscore after hex prefix",
			input:       "[0x_FF]string",
			expectError: true,
		},
		{
			name:        "array with underscore after binary prefix",
			input:       "[0b_11]string",
			expectError: true,
		},
		{
			name:        "array with underscore after octal prefix",
			input:       "[0o_77]string",
			expectError: true,
		},
		{
			name:        "array with trailing underscore in hex",
			input:       "[0xFF_]string",
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

		// Unicode handling tests - these will fail with current byte-based implementation
		{
			name:     "simple Unicode identifier - Chinese",
			input:    "世界",
			expected: &NamedType{Name: "世界"},
		},
		{
			name:     "simple Unicode identifier - Greek",
			input:    "αβγ",
			expected: &NamedType{Name: "αβγ"},
		},
		{
			name:     "simple Unicode identifier - Latin with diacritics",
			input:    "café",
			expected: &NamedType{Name: "café"},
		},
		{
			name:     "qualified type with Unicode package",
			input:    "github.com/用户/项目.类型",
			expected: &NamedType{ImportPath: "github.com/用户/项目", Name: "类型"},
		},
		{
			name:     "qualified type with Unicode in path component",
			input:    "example.com/café/lib.Type",
			expected: &NamedType{ImportPath: "example.com/café/lib", Name: "Type"},
		},
		{
			name:     "pointer to Unicode type",
			input:    "*世界",
			expected: &PointerType{Elem: &NamedType{Name: "世界"}},
		},
		{
			name:     "slice of Unicode type",
			input:    "[]αβγ",
			expected: &SliceType{Elem: &NamedType{Name: "αβγ"}},
		},
		{
			name:     "array of Unicode type",
			input:    "[10]café",
			expected: &ArrayType{Size: 10, Elem: &NamedType{Name: "café"}},
		},
		{
			name:     "map with Unicode types",
			input:    "map[string]世界",
			expected: &MapType{Key: &NamedType{Name: "string"}, Value: &NamedType{Name: "世界"}},
		},
		{
			name:     "map with Unicode key",
			input:    "map[αβγ]string",
			expected: &MapType{Key: &NamedType{Name: "αβγ"}, Value: &NamedType{Name: "string"}},
		},
		{
			name:     "complex nested type with Unicode",
			input:    "*[]map[string]αβγ",
			expected: &PointerType{Elem: &SliceType{Elem: &MapType{Key: &NamedType{Name: "string"}, Value: &NamedType{Name: "αβγ"}}}},
		},
		{
			name:     "mixed ASCII and Unicode",
			input:    "mypackage.世界Type",
			expected: &NamedType{ImportPath: "mypackage", Name: "世界Type"},
		},
		{
			name:     "Unicode with numbers",
			input:    "Type123世界",
			expected: &NamedType{Name: "Type123世界"},
		},
		{
			name:     "package path with Unicode directory",
			input:    "github.com/世界/package.MyType",
			expected: &NamedType{ImportPath: "github.com/世界/package", Name: "MyType"},
		},
		{
			name:     "right-to-left script",
			input:    "مرحبا",
			expected: &NamedType{Name: "مرحبا"},
		},
		{
			name:     "cyrillic script",
			input:    "Привет",
			expected: &NamedType{Name: "Привет"},
		},
		{
			name:     "korean script",
			input:    "안녕하세요",
			expected: &NamedType{Name: "안녕하세요"},
		},
		{
			name:     "japanese script",
			input:    "こんにちは",
			expected: &NamedType{Name: "こんにちは"},
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
