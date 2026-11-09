package tun

import (
	"bytes"
	"testing"
)

// Use a real established stack connection and inspect emitted IP packets,
// without relying on sleeps or wall-clock retransmission deadlines.
func TestNHOPTCPReorderingDoesNotRetransmitEarly(t *testing.T) {
	for _, src := range []string{"10.0.0.1:1234", "[fd00::1]:1234"} {
		t.Run(src, func(t *testing.T) {
			p, c := nhopLossPeer(t, src)
			for n := uint32(1); n <= 3; n++ {
				s := nhopSelectiveACK(p.ack, [2]uint32{100, 100 + n})
				s.window = p.window
				c.mu.Lock()
				c.acknowledge(s)
				c.mu.Unlock()
				if n < 3 {
					select {
					case packet := <-p.sent:
						t.Fatalf("premature retransmission after SACK %d: seq=%d", n, p.segment(packet).seq)
					default:
					}
				}
			}
			s := p.receive()
			if s.seq != p.ack || !bytes.Equal(s.payload, bytes.Repeat([]byte{0x5a}, 100)) {
				t.Fatal("three distinct small SACKs did not retransmit the first missing segment")
			}
		})
	}
}

func nhopLossPeer(t *testing.T, src string) (*peer, *tcpConn) {
	t.Helper()
	dst := "10.0.0.2:80"
	if src[0] == '[' {
		dst = "[fd00::2]:80"
	}
	p, l := newPeer(t, src, dst)
	conns := make(chan accepted, 1)
	newTestStack(t, l, accept(conns))
	p.connect([]byte{2, 4, 0, 100, 4, 2, 0, 0})
	c := take(t, conns).conn.(*tcpConn)
	write(t, c, bytes.Repeat([]byte{0x5a}, 1000))
	for range 10 {
		p.receive()
	}
	// Keep the background RTO out of this synchronous ACK-policy test.
	c.mu.Lock()
	c.retransmitTicks = tcpRetransmitTicksMost
	c.mu.Unlock()
	return p, c
}
