package packetqueue

import "encoding/binary"

type flowKey struct {
	version, protocol   byte
	source, destination [16]byte
	ports               [4]byte
}

// Base-header TCP/UDP uses a directional five-tuple. Fragments and extension
// headers use address/protocol buckets, without interpreting transport bytes.
// Malformed packets share a bucket but are never modified or discarded here.
func classify(packet []byte) flowKey {
	if len(packet) == 0 {
		return flowKey{}
	}
	switch packet[0] >> 4 {
	case 4:
		return ipv4Flow(packet)
	case 6:
		return ipv6Flow(packet)
	default:
		return flowKey{}
	}
}

func ipv4Flow(packet []byte) flowKey {
	if len(packet) < 20 {
		return flowKey{}
	}
	header, size := int(packet[0]&15)*4, int(binary.BigEndian.Uint16(packet[2:4]))
	if header < 20 || size < header || size > len(packet) {
		return flowKey{}
	}
	key := flowKey{version: 4, protocol: packet[9]}
	copy(key.source[:], packet[12:16])
	copy(key.destination[:], packet[16:20])
	if binary.BigEndian.Uint16(packet[6:8])&0x3fff == 0 {
		key.transport(packet[header:size])
	}
	return key
}

func ipv6Flow(packet []byte) flowKey {
	if len(packet) < 40 {
		return flowKey{}
	}
	size := 40 + int(binary.BigEndian.Uint16(packet[4:6]))
	if size > len(packet) {
		return flowKey{}
	}
	key := flowKey{version: 6, protocol: packet[6]}
	copy(key.source[:], packet[8:24])
	copy(key.destination[:], packet[24:40])
	key.transport(packet[40:size])
	return key
}

func (key *flowKey) transport(segment []byte) {
	if (key.protocol == 6 || key.protocol == 17) && len(segment) >= 4 {
		copy(key.ports[:], segment[:4])
	}
}
