package packettrace

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"net/netip"
)

// The UDP body is identified by its digest, never its contents. This lets the
// synthetic experiment correlate a datagram across queues and IP TTL changes.
func nhopUDPHeader(packet []byte) (string, bool) {
	src, dst, segment := nhopIPSegment(packet, 17)
	if len(segment) < 8 {
		return "", false
	}
	size := int(binary.BigEndian.Uint16(segment[4:6]))
	if size < 8 || size > len(segment) {
		return "", false
	}
	source := netip.AddrPortFrom(src, binary.BigEndian.Uint16(segment[:2]))
	destination := netip.AddrPortFrom(dst, binary.BigEndian.Uint16(segment[2:4]))
	return fmt.Sprintf("src=%s dst=%s udp=%x payload=%d digest=%x", source, destination, segment[:8], size-8, sha256.Sum256(segment[8:size])), true
}
