// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package utils

import (
	"net"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

// GetFreePort returns a random free port.
func GetFreePort(t *testing.T) string {
	t.Helper()
	li, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := li.Addr()
	require.NoError(t, li.Close())
	return strconv.Itoa(addr.(*net.TCPAddr).Port)
}
