//go:build linux

package masque

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"vpn-script/internal/platform/safeio"
)

// The echo receiver must hold one entire burst, including kernel skb overhead.
// SO_RCVBUFFORCE affects only this socket, never the namespace or host sysctls.
func prepareOriginUDP(t *testing.T, connection *net.UDPConn) error {
	t.Helper()
	requireIsolation(t)
	workers := boundedEnvironment(t, "NHOP_MASQUE_WORKERS", 8, 64)
	packets := boundedEnvironment(t, "NHOP_MASQUE_PACKETS", 32, 256)
	requested := max(128<<10, workers*packets*4096)
	actual, err := originReceiveBuffer(connection, requested)
	if err != nil {
		return err
	}
	// Linux reports twice the requested bytes for socket accounting overhead.
	if actual != 2*requested {
		return fmt.Errorf("origin receive buffer was clamped: requested=%d actual=%d", requested, actual)
	}
	return safeio.JSON(filepath.Join(os.Getenv("NHOP_MASQUE_REPORT"), "origin-buffer.json"), map[string]int{
		"workers": workers, "packets_per_worker": packets, "accounting_bytes_per_packet": 4096,
		"requested_bytes": requested, "observed_bytes": actual,
	})
}

func originReceiveBuffer(connection *net.UDPConn, requested int) (int, error) {
	raw, err := connection.SyscallConn()
	if err != nil {
		return 0, err
	}
	var actual int
	var socketErr error
	err = raw.Control(func(fd uintptr) {
		socketErr = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_RCVBUFFORCE, requested)
		if socketErr == nil {
			actual, socketErr = syscall.GetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_RCVBUF)
		}
	})
	if err != nil {
		return 0, err
	}
	return actual, socketErr
}
