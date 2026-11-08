package packetqueue

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func ipPacket(version byte) []byte {
	packet := make([]byte, 44)
	packet[0], packet[9] = 0x45, 6
	copy(packet[12:20], []byte{192, 0, 2, 1, 198, 51, 100, 2})
	binary.BigEndian.PutUint16(packet[2:4], uint16(len(packet)))
	header := 20
	if version == 6 {
		packet = make([]byte, 64)
		packet[0], packet[6], packet[8], packet[23], packet[24], packet[39] = 0x60, 6, 0xfd, 1, 0xfd, 2
		binary.BigEndian.PutUint16(packet[4:6], 24)
		header = 40
	}
	copy(packet[header:header+4], []byte{0x9c, 0x40, 0x46, 0xa0})
	return packet
}

func TestFlowIdentityAndTruncatedPackets(t *testing.T) {
	for _, version := range []byte{4, 6} {
		packet := ipPacket(version)
		key := classify(packet)
		if key.version != version || key.protocol != 6 || key.ports != [4]byte{0x9c, 0x40, 0x46, 0xa0} {
			t.Fatalf("wrong TCP five-tuple: %+v", key)
		}
		for size := range len(packet) {
			if classify(packet[:size]) != (flowKey{}) {
				t.Fatalf("recognized truncated IPv%d packet at %d", version, size)
			}
		}
		header := 20
		if version == 6 {
			header = 40
		}
		changed := bytes.Clone(packet)
		changed[header+1]++
		if classify(changed) == key {
			t.Fatal("different transport flow coalesced")
		}
		changed = bytes.Clone(packet)
		changed[12]++
		if classify(changed) == key {
			t.Fatal("different address coalesced")
		}
	}
}

func TestFragmentsAndExtensionsDoNotReadTransportPorts(t *testing.T) {
	fragment := ipPacket(4)
	fragment[6] = 0x20
	key := classify(fragment)
	if key.version != 4 || key.ports != [4]byte{} {
		t.Fatal("IPv4 fragment treated as an independent transport header")
	}
	extension := ipPacket(6)
	extension[6] = 0
	key = classify(extension)
	if key.version != 6 || key.protocol != 0 || key.ports != [4]byte{} {
		t.Fatal("IPv6 extension treated as a transport header")
	}
	for first := range 256 {
		packet := ipPacket(4)
		packet[0] = byte(first)
		for size := range len(packet) {
			classify(packet[:size]) // Every version/header-length combination must be bounds-safe.
		}
	}
}
