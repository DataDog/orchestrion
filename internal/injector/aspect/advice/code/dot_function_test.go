// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package code

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestResolveInterfaceTypeByName tests the resolveInterfaceTypeByName function with various interface name formats.
func TestResolveInterfaceTypeByName(t *testing.T) {
	// Test cases for different interface types
	testCases := []struct {
		name           string
		interfaceName  string
		shouldSucceed  bool
		expectedMethod string
	}{
		{
			name:           "stdlib interface io.Reader",
			interfaceName:  "io.Reader",
			shouldSucceed:  true,
			expectedMethod: "Read",
		},
		{
			name:           "built-in error interface",
			interfaceName:  "error",
			shouldSucceed:  true,
			expectedMethod: "Error",
		},
		{
			name:          "invalid interface name",
			interfaceName: "123invalid",
			shouldSucceed: false,
		},
		{
			name:          "nonexistent interface",
			interfaceName: "nonexistent.Interface",
			shouldSucceed: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			iface, err := resolveInterfaceTypeByName(tc.interfaceName)

			if !tc.shouldSucceed {
				require.Error(t, err)
				require.Nil(t, iface)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, iface)

			if tc.expectedMethod != "" {
				found := false
				for i := 0; i < iface.NumMethods(); i++ {
					if iface.Method(i).Name() == tc.expectedMethod {
						found = true
						break
					}
				}

				assert.True(t, found, "Interface does not contain expected method %s", tc.expectedMethod)
			}
		})
	}
}
