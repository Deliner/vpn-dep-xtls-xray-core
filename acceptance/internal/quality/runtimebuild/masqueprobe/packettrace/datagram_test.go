package packettrace

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"strings"
	"testing"
)

func nhopTestDatagram(version int) []byte {
	packet := nhopTestPacket(version)
	header := 20
	if version == 6 {
		header = 40
		packet[6] = 17
	} else {
		packet[9] = 17
	}
	udp := packet[header:]
	binary.BigEndian.PutUint16(udp[4:6], uint16(len(udp)))
	return packet
}

func TestNHOPDatagramHeaderKeepsIdentityWithoutPayload(t *testing.T) {
	for _, version := range []int{4, 6} {
		packet := nhopTestDatagram(version)
		original := bytes.Clone(packet)
		header, ok := nhopUDPHeader(packet)
		_, _, segment := nhopIPSegment(packet, 17)
		want := fmt.Sprintf("payload=%d digest=%x", len(segment)-8, sha256.Sum256(segment[8:]))
		if !ok || !strings.Contains(header, want) || !bytes.Equal(packet, original) {
			t.Fatalf("datagram identity or bytes changed: %s", header)
		}
		if strings.Contains(header, "private") || strings.Contains(header, "70726976617465") {
			t.Fatal("datagram observer disclosed payload")
		}
		for size := range len(packet) {
			if _, ok := nhopUDPHeader(packet[:size]); ok {
				t.Fatalf("accepted truncated IPv%d datagram at %d", version, size)
			}
		}
	}
}

func TestNHOPDatagramHeaderRejectsAmbiguousPackets(t *testing.T) {
	for _, change := range []func([]byte){
		func(b []byte) { b[9] = 6 },
		func(b []byte) { b[6] = 0x20 },
		func(b []byte) { b[7] = 1 },
		func(b []byte) { b[24], b[25] = 0, 7 },
		func(b []byte) { b[24], b[25] = 0xff, 0xff },
	} {
		packet := nhopTestDatagram(4)
		change(packet)
		if _, ok := nhopUDPHeader(packet); ok {
			t.Fatal("accepted fragmented, malformed or non-UDP packet")
		}
	}
	packet := nhopTestDatagram(6)
	packet[6] = 0
	if _, ok := nhopUDPHeader(packet); ok {
		t.Fatal("interpreted an IPv6 extension header as UDP")
	}
}
