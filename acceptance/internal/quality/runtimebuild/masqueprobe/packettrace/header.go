package packettrace

import (
	"encoding/binary"
	"fmt"
	"net/netip"
)

// Only complete, unfragmented IPv4 or base-header IPv6 TCP is observed. Other
// packets still follow the original engine path, without diagnostic output.
func nhopTCPHeader(packet []byte) (string, bool) {
	src, dst, segment := nhopTCPSegment(packet)
	if len(segment) < 20 {
		return "", false
	}
	header := int(segment[12]>>4) * 4
	if header < 20 || header > len(segment) {
		return "", false
	}
	source := netip.AddrPortFrom(src, binary.BigEndian.Uint16(segment[:2]))
	destination := netip.AddrPortFrom(dst, binary.BigEndian.Uint16(segment[2:4]))
	return fmt.Sprintf("src=%s dst=%s tcp=%x payload=%d", source, destination, segment[:header], len(segment)-header), true
}

func nhopTCPSegment(packet []byte) (src, dst netip.Addr, segment []byte) {
	if len(packet) == 0 {
		return
	}
	switch packet[0] >> 4 {
	case 4:
		if len(packet) < 20 || packet[9] != 6 || binary.BigEndian.Uint16(packet[6:8])&0x3fff != 0 {
			return
		}
		header, total := int(packet[0]&15)*4, int(binary.BigEndian.Uint16(packet[2:4]))
		if header < 20 || total < header || total > len(packet) {
			return
		}
		src, dst = netip.AddrFrom4([4]byte(packet[12:16])), netip.AddrFrom4([4]byte(packet[16:20]))
		return src, dst, packet[header:total]
	case 6:
		if len(packet) < 40 || packet[6] != 6 {
			return
		}
		total := 40 + int(binary.BigEndian.Uint16(packet[4:6]))
		if total > len(packet) {
			return
		}
		src, dst = netip.AddrFrom16([16]byte(packet[8:24])), netip.AddrFrom16([16]byte(packet[24:40]))
		return src, dst, packet[40:total]
	}
	return
}
