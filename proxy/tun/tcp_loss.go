package tun

import "encoding/binary"

// acknowledgeSACK records only valid in-flight ranges. For a SACK-capable
// peer a duplicate ACK means newly SACKed data, not another copy of a report
// (RFC 6675 section 2). Cumulative progress already reset dupAcks in acknowledge.
// The ACK may also carry payload or a changed receive window. c.mu is held.
func (c *tcpConn) acknowledgeSACK(options []byte) {
	before := c.sackedBytes()
	for kind, value, rest := nextOption(options); kind != 0; kind, value, rest = nextOption(rest) {
		for ; kind == 5 && len(value) >= 8; value = value[8:] {
			start := binary.BigEndian.Uint32(value) - c.sndUna
			end := binary.BigEndian.Uint32(value[4:]) - c.sndUna
			if start != 0 && start < end && end <= c.sndNxt-c.sndUna {
				c.sackAdd(c.sndUna+start, c.sndUna+end)
			}
		}
	}
	if !c.recovering && c.sackedBytes() > before && c.dupAcks < 3 {
		c.dupAcks++
	}
}

// sackedBytes counts the union kept by sackAdd, so overlaps, adjacent
// blocks and repeated reports cannot inflate the loss evidence. c.mu is held.
func (c *tcpConn) sackedBytes() uint32 {
	var total uint32
	for _, block := range c.sacked {
		total += block[1] - block[0]
	}
	return total
}

// lossIndicated applies the fast-recovery entry threshold from RFC 6675
// sections 4 and 5: three new SACK reports, three disjoint ranges, or more
// than two MSS of SACKed bytes above the first missing byte. Without SACK,
// acknowledge counts conventional duplicate ACKs instead. c.mu is held.
func (c *tcpConn) lossIndicated() bool {
	if c.dupAcks >= 3 {
		return true
	}
	return c.sack && (len(c.sacked) >= 3 || c.sackedBytes() > 2*uint32(c.mss))
}
