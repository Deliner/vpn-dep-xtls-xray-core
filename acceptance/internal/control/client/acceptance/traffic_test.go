package acceptance

import (
	"net"
	"testing"
)

func udpOrigin(t *testing.T) {
	t.Helper()
	listener, err := net.ListenPacket("udp", "198.18.0.2:18081")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { listener.Close() })
	go func() {
		buffer := make([]byte, 1024)
		for {
			n, address, err := listener.ReadFrom(buffer)
			if err != nil {
				return
			}
			_, _ = listener.WriteTo(buffer[:n], address)
		}
	}()
}

func checkTraffic(t *testing.T, c *controller, allowed bool) {
	t.Helper()
	for _, operation := range []string{"probe", "probe6", "probe-udp"} {
		result := c.call(t, operation)
		if (result.Error == "") != allowed {
			t.Fatalf("%s allowed=%v: %s", operation, allowed, result.Error)
		}
	}
}
