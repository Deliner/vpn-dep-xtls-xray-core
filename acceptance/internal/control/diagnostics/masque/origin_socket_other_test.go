//go:build !linux

package masque

import (
	"fmt"
	"net"
	"testing"
)

func prepareOriginUDP(_ *testing.T, _ *net.UDPConn) error {
	return fmt.Errorf("the isolated MASQUE origin requires Linux socket accounting")
}
