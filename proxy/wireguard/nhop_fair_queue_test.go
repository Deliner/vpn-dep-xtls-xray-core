package wireguard

import (
	"encoding/binary"
	"net/netip"
	"testing"
	"time"
)

// A bulk UDP burst must retain every packet without placing a new TCP control
// flow behind the entire burst. FIFO remains mandatory inside each flow.
func TestNHOPNetTUNMixedFlowScheduling(t *testing.T) {
	device := nhopSchedulingDevice(t)
	for flow := range 8 {
		for sequence := range 32 {
			packet := nhopSchedulingPacket(17, uint16(40000+flow), byte(sequence))
			if err := device.WritePacket(packet[:20], packet[20:]); err != nil {
				t.Fatal(err)
			}
		}
	}
	ack := nhopSchedulingPacket(6, 50000, 0)
	if err := device.WritePacket(ack[:20], ack[20:]); err != nil {
		t.Fatal(err)
	}
	position := nhopReadScheduledBurst(t, device)
	if position > 9 {
		t.Fatalf("TCP control flow waited behind bulk UDP: position=%d, fair bound=9", position)
	}
	t.Logf("TCP control position=%d; all 256 UDP packets retained in per-flow order", position)
}

func nhopSchedulingDevice(t *testing.T) *netTun {
	t.Helper()
	device, _, err := CreateNetTUN([]netip.Addr{netip.MustParseAddr("10.66.0.2")}, nil, 1280, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { device.Close() })
	deadline := time.AfterFunc(2*time.Second, func() { device.Close() })
	t.Cleanup(func() { deadline.Stop() })
	return device.(*netTun)
}

func nhopSchedulingPacket(protocol byte, port uint16, sequence byte) []byte {
	packet := make([]byte, 40)
	packet[0], packet[8], packet[9] = 0x45, 64, protocol
	binary.BigEndian.PutUint16(packet[2:4], uint16(len(packet)))
	copy(packet[12:20], []byte{10, 66, 0, 2, 198, 18, 0, 2})
	binary.BigEndian.PutUint16(packet[20:22], port)
	binary.BigEndian.PutUint16(packet[22:24], 18080)
	if protocol == 6 {
		packet[32], packet[33] = 0x50, 0x10
	} else {
		binary.BigEndian.PutUint16(packet[24:26], 20)
	}
	packet[39] = sequence
	return packet
}

func nhopReadScheduledBurst(t *testing.T, device *netTun) int {
	t.Helper()
	next := map[uint16]byte{}
	controlPosition := 0
	for position := 1; position <= 257; position++ {
		buffer, sizes := make([]byte, 1280), []int{0}
		if n, err := device.Read([][]byte{buffer}, sizes, 0); n != 1 || err != nil || sizes[0] != 40 {
			t.Fatalf("missing packet at %d: n=%d size=%d err=%v", position, n, sizes[0], err)
		}
		port := binary.BigEndian.Uint16(buffer[20:22])
		if buffer[39] != next[port] {
			t.Fatalf("flow %d reordered or duplicated: got=%d want=%d", port, buffer[39], next[port])
		}
		next[port]++
		if port == 50000 {
			controlPosition = position
		}
	}
	if controlPosition == 0 || next[50000] != 1 || len(next) != 9 {
		t.Fatal("missing or duplicate flow")
	}
	for port := uint16(40000); port < 40008; port++ {
		if next[port] != 32 {
			t.Fatalf("UDP flow %d lost packets: %d", port, next[port])
		}
	}
	return controlPosition
}
