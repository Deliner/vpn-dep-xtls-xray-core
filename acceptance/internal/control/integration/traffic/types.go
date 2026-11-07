// Package traffic provides a barrier-controlled TCP/UDP lab for integration tests.
// It has no dependency on VPN engines or topology compilation.
package traffic

import (
	"context"
	"net"
	"sync"
)

type Endpoint struct {
	ID   string
	Port int
}

type Result struct {
	TCPRounds int    `json:"tcp_rounds"`
	UDPRounds int    `json:"udp_rounds"`
	Error     string `json:"error,omitempty"`
}

type Report struct {
	Phase             string            `json:"phase"`
	Profiles          int               `json:"profiles"`
	RequiredRounds    int               `json:"required_rounds"`
	BarrierRounds     int               `json:"barrier_rounds"`
	ConcurrentStreams int               `json:"concurrent_tcp_udp_streams"`
	TCPBytes          int               `json:"tcp_bytes_per_round"`
	UDPBytes          int               `json:"udp_bytes_per_round"`
	DurationMS        int64             `json:"duration_ms"`
	Routes            map[string]Result `json:"routes"`
	Missing           []string          `json:"missing_arrivals,omitempty"`
}

type round struct {
	seen  map[string]bool
	ready chan struct{}
}

type barrier struct {
	mu        sync.Mutex
	expected  map[string]bool
	rounds    []*round
	completed int
	peak      int
}

type origin struct {
	context context.Context
	cancel  context.CancelFunc
	tcp     net.Listener
	udp     *net.UDPConn
	gate    *barrier
	workers sync.WaitGroup
}

type session struct {
	tcp     net.Conn
	control net.Conn
	udp     net.Conn
	stop    func() bool
}

type outcome struct {
	id     string
	result Result
}

// UDPProbe owns both connections of a persistent SOCKS UDP association.
type UDPProbe struct {
	control    net.Conn
	connection net.Conn
	stop       func() bool
}
