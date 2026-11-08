package packettrace

import (
	"strings"
	"testing"
)

func TestNHOPPacketTraceHasIdentityAndNoPayload(t *testing.T) {
	packet := nhopTestPacket(4)
	line, ok := nhopTCPObservation("submit", &packet, packet)
	if !ok {
		t.Fatal("valid packet has no observation")
	}
	for _, field := range []string{"NHOP_INNER_TCP unix_ns=", " pid=", " conn=0x", " phase=submit", "src=192.0.2.1:40000", "dst=198.51.100.2:18000", "payload=36"} {
		if !strings.Contains(line, field) {
			t.Fatalf("missing observation field %q: %s", field, line)
		}
	}
	if strings.Contains(line, "private") || strings.Contains(line, "70726976617465") || strings.ContainsAny(line, "\n\r") {
		t.Fatal("trace disclosed payload or lost line framing")
	}
	if _, ok := nhopTCPObservation("deliver", &packet, nil); ok {
		t.Fatal("invalid packet generated an observation")
	}
	nhopTraceTCP("submit", &packet, packet)
}

func TestNHOPDeviceTraceHasQueueAndPacketIdentity(t *testing.T) {
	packet := nhopTestPacket(4)
	line, ok := nhopDeviceObservation("offer", &packet, &packet, packet, 27)
	if !ok {
		t.Fatal("valid packet has no device observation")
	}
	for _, field := range []string{"NHOP_DEVICE_TCP unix_ns=", " pid=", " device=0x", " packet=0x", " phase=offer queued=27", "payload=36"} {
		if !strings.Contains(line, field) {
			t.Fatalf("missing observation field %q: %s", field, line)
		}
	}
	if strings.Contains(line, "private") || strings.Contains(line, "70726976617465") {
		t.Fatal("device trace disclosed payload")
	}
	nhopTraceDevice("offer", &packet, &packet, packet, 27)
}
