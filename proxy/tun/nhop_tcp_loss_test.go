package tun

import (
	"encoding/binary"
	"fmt"
	"testing"
)

// RFC 6675 sections 2, 4 and 5 distinguish newly SACKed data from
// duplicated reports and require evidence beyond one reordered segment.
func TestNHOPTCPReorderingLossThreshold(t *testing.T) {
	for _, base := range []uint32{1000, 0xffffff80} {
		for _, tc := range nhopSACKCases() {
			t.Run(fmt.Sprintf("%08x/%s", base, tc.name), func(t *testing.T) {
				c := nhopLossConnection(base, true)
				s := nhopSelectiveACK(base, tc.blocks...)
				for range 4 {
					c.acknowledge(s)
					nhopCheckRecovery(t, c, tc.lost)
				}
			})
		}
	}
}

type nhopSACKCase struct {
	name   string
	blocks [][2]uint32
	lost   bool
}

func nhopSACKCases() []nhopSACKCase {
	return []nhopSACKCase{
		{name: "no-sack"},
		{name: "one-segment", blocks: [][2]uint32{{100, 200}}},
		{name: "two-segments", blocks: [][2]uint32{{100, 300}}},
		{name: "two-islands", blocks: [][2]uint32{{100, 101}, {200, 201}}},
		{name: "overlap", blocks: [][2]uint32{{100, 300}, {150, 250}}},
		{name: "adjacent", blocks: [][2]uint32{{100, 150}, {150, 200}, {200, 250}}},
		{name: "past-flight", blocks: [][2]uint32{{100, 1001}}},
		{name: "before-ack", blocks: [][2]uint32{{0xffffff00, 100}}},
		{name: "more-than-two", blocks: [][2]uint32{{100, 301}}, lost: true},
		{name: "three-islands", blocks: [][2]uint32{{100, 101}, {200, 201}, {300, 301}}, lost: true},
	}
}

func TestNHOPTCPNewSmallSACKsTriggerRecovery(t *testing.T) {
	for _, base := range []uint32{1000, 0xffffff80} {
		c := nhopLossConnection(base, true)
		for n := uint32(1); n <= 3; n++ {
			s := nhopSelectiveACK(base, [2]uint32{100, 100 + n})
			// SACK duplicate ACKs concern newly reported data, even when
			// the window changes or the ACK also carries reverse data.
			s.window -= uint16(n)
			s.payload = []byte{1}
			for range 4 {
				c.acknowledge(s)
				nhopCheckRecovery(t, c, n == 3)
			}
		}
	}
}

func TestNHOPTCPSelectiveACKAfterCumulativeProgress(t *testing.T) {
	c := nhopLossConnection(1000, true)
	c.acknowledge(nhopSelectiveACK(1000, [2]uint32{200, 201}))
	c.acknowledge(nhopSelectiveACK(1000, [2]uint32{200, 202}))
	for n := uint32(1); n <= 3; n++ {
		c.acknowledge(nhopSelectiveACK(1100, [2]uint32{100, 102 + n}))
		if c.recovering != (n == 3) {
			t.Fatalf("new SACK #%d after cumulative progress: recovering=%t", n, c.recovering)
		}
	}
}

func TestNHOPTCPNonSACKDuplicateThreshold(t *testing.T) {
	c := nhopLossConnection(1000, false)
	s := nhopSelectiveACK(1000)
	for n := 1; n <= 3; n++ {
		c.acknowledge(s)
		nhopCheckRecovery(t, c, n == 3)
	}
}

func nhopLossConnection(base uint32, sack bool) *tcpConn {
	return &tcpConn{
		mss: 100, window: 65535, sndWnd: 65535, sack: sack,
		sndUna: base, sndNxt: base + 1000, sndBase: base,
		cwnd: 1000, ssthresh: 65535, resent: base, lost: base,
	}
}

func nhopSelectiveACK(base uint32, blocks ...[2]uint32) segment {
	s := segment{ack: base, flags: tcpAck, window: 65535}
	if len(blocks) == 0 {
		return s
	}
	s.options = []byte{1, 1, 5, byte(2 + 8*len(blocks))}
	for _, b := range blocks {
		s.options = binary.BigEndian.AppendUint32(s.options, base+b[0])
		s.options = binary.BigEndian.AppendUint32(s.options, base+b[1])
	}
	return s
}

func nhopCheckRecovery(t *testing.T, c *tcpConn, lost bool) {
	t.Helper()
	want := uint32(1000)
	if lost {
		want = 500
	}
	if c.recovering != lost || c.cwnd != want {
		t.Fatalf("recovering=%t cwnd=%d; want recovering=%t cwnd=%d", c.recovering, c.cwnd, lost, want)
	}
}
