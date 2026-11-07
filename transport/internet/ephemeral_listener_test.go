//go:build linux || darwin

package internet_test

import (
	"context"
	"errors"
	"net"
	"syscall"
	"testing"

	"github.com/xtls/xray-core/transport/internet"
)

func TestAutomaticUDPListenerIsExclusive(t *testing.T) {
	listener := new(internet.DefaultListener)
	first, err := listener.ListenPacket(context.Background(), &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	second, err := listener.ListenPacket(context.Background(), first.LocalAddr(), nil)
	if second != nil {
		second.Close()
		t.Fatalf("automatic UDP address %s was also assigned to an unrelated listener", first.LocalAddr())
	}
	if !errors.Is(err, syscall.EADDRINUSE) {
		t.Fatalf("expected occupied address refusal, got %v", err)
	}
}
