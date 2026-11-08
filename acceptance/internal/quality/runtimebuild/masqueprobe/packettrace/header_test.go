package packettrace

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"strings"
	"testing"
)

func nhopTestPacket(version int) []byte {
	tcp, _ := hex.DecodeString("9c40465001020304050607086012100000000000020405b4")
	tcp = append(tcp, []byte("private-payload-must-never-be-logged")...)
	ip := make([]byte, 20)
	ip[0], ip[8], ip[9] = 0x45, 64, 6
	copy(ip[12:], []byte{192, 0, 2, 1, 198, 51, 100, 2})
	binary.BigEndian.PutUint16(ip[2:], uint16(len(ip)+len(tcp)))
	if version == 6 {
		ip = make([]byte, 40)
		ip[0], ip[6], ip[7], ip[8], ip[23], ip[24], ip[39] = 0x60, 6, 64, 0xfd, 1, 0xfd, 2
		binary.BigEndian.PutUint16(ip[4:], uint16(len(tcp)))
	}
	return append(ip, tcp...)
}

func TestNHOPPacketHeaderContainsOnlyCompleteTCPHeader(t *testing.T) {
	for _, version := range []int{4, 6} {
		packet := nhopTestPacket(version)
		original := bytes.Clone(packet)
		header, ok := nhopTCPHeader(packet)
		if !ok || !strings.Contains(header, "tcp=9c40465001020304050607086012100000000000020405b4 payload=36") {
			t.Fatalf("missing TCP options or wrong payload size: %s", header)
		}
		if strings.Contains(header, "private") || strings.Contains(header, "70726976617465") || !bytes.Equal(packet, original) {
			t.Fatal("observer disclosed payload or mutated the packet")
		}
		for size := range len(packet) {
			if _, ok := nhopTCPHeader(packet[:size]); ok {
				t.Fatalf("accepted truncated IPv%d packet at %d", version, size)
			}
		}
	}
}

func TestNHOPPacketHeaderRejectsAmbiguousPackets(t *testing.T) {
	for _, change := range []func([]byte){
		func(b []byte) { b[9] = 17 },
		func(b []byte) { b[6] = 0x20 },
		func(b []byte) { b[7] = 1 },
		func(b []byte) { b[0] = 0x44 },
		func(b []byte) { b[2], b[3] = 0, 19 },
		func(b []byte) { b[32] = 0x40 },
		func(b []byte) { b[32], b[3] = 0xf0, 79 },
	} {
		packet := nhopTestPacket(4)
		change(packet)
		if _, ok := nhopTCPHeader(packet); ok {
			t.Fatal("accepted malformed, fragmented or non-TCP packet")
		}
	}
	packet := nhopTestPacket(6)
	packet[6] = 0 // IPv6 extension headers are outside this diagnostic's scope.
	if _, ok := nhopTCPHeader(packet); ok {
		t.Fatal("interpreted an IPv6 extension header as TCP")
	}
}
