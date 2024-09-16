package utils

import (
	"net"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

func GetFreePort(t *testing.T) string {
	t.Helper()
	li, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := li.Addr()
	require.NoError(t, li.Close())
	return strconv.Itoa(addr.(*net.TCPAddr).Port)
}
