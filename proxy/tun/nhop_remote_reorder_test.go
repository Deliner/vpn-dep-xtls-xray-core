package tun

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/netip"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/xtls/xray-core/common/net"
)

// Isolate the remote TCP stack from QUIC and HTTP/3: delivery is loss-free,
// but neighbouring IP packets may arrive out of order after 15..25 milliseconds.
func TestNHOPRemoteTCPReordering(t *testing.T) {
	for _, jitter := range []bool{false, true} {
		t.Run(fmt.Sprintf("jitter-%t", jitter), func(t *testing.T) {
			for batch := range 3 {
				t.Run(fmt.Sprint(batch), func(t *testing.T) {
					nhopRemoteExchange(t, jitter)
				})
			}
		})
	}
}

type nhopEchoHandler struct{}

func (nhopEchoHandler) HandleConnection(conn net.Conn, destination net.Destination) {
	echo(conn, destination)
}

func nhopRemoteExchange(t *testing.T, jitter bool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	left, right := newLink(), newLink()
	forward := nhopDelayedLink(ctx, left, right, jitter)
	backward := nhopDelayedLink(ctx, right, left, jitter)
	client := nhopRemoteStack(t, left, "198.18.0.1", nil)
	_ = nhopRemoteStack(t, right, "198.18.0.2", nhopEchoHandler{})
	results := make(chan error, 8)
	start := make(chan struct{})
	for worker := range 8 {
		go func() {
			<-start
			results <- nhopTCPExchange(ctx, client, worker)
		}()
	}
	close(start)
	for range 8 {
		if err := <-results; err != nil {
			t.Error(err)
		}
	}
	t.Logf("within-flow reordering: forward=%d backward=%d", forward.Load(), backward.Load())
	if jitter && forward.Load()+backward.Load() == 0 {
		t.Error("jitter did not demonstrably reorder packets of a TCP flow")
	}
}

func nhopDelayedLink(ctx context.Context, source, destination *link, jitter bool) *atomic.Uint64 {
	var sequence, reordered atomic.Uint64
	var delivery sync.Mutex
	last := map[string]uint64{}
	source.out = func(packet []byte) {
		position := sequence.Add(1)
		delay := 20 * time.Millisecond
		if jitter {
			delay = time.Duration(15+position%11) * time.Millisecond
		}
		time.AfterFunc(delay, func() {
			delivery.Lock()
			defer delivery.Unlock()
			select {
			case destination.in <- packet:
				flow := string(packet[12:24]) // IPv4 addresses and TCP ports.
				if position < last[flow] {
					reordered.Add(1)
				}
				last[flow] = position
			case <-ctx.Done():
			case <-destination.done:
			}
		})
	}
	return &reordered
}

func nhopRemoteStack(t *testing.T, link *link, address string, handler ConnectionHandler) Stack {
	t.Helper()
	stack, err := NewStack(t.Context(), StackOptions{
		Tun: link, MTU: 1280, Remote: true,
		Addresses: []netip.Addr{netip.MustParseAddr(address)},
	}, handler)
	if err != nil {
		t.Fatal(err)
	}
	if err := stack.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		stack.Close()
		link.Close()
	})
	return stack
}

func nhopTCPExchange(ctx context.Context, stack Stack, worker int) error {
	conn, err := stack.DialTCP(ctx, netip.MustParseAddrPort("198.18.0.2:18080"))
	if err != nil {
		return fmt.Errorf("worker %d dial: %w", worker, err)
	}
	defer conn.Close()
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	defer stop()
	payload := bytes.Repeat([]byte{byte(worker + 1)}, 32768)
	if _, err := io.Copy(conn, bytes.NewReader(payload)); err != nil {
		return fmt.Errorf("worker %d send: %w", worker, err)
	}
	reply := make([]byte, len(payload))
	if _, err := io.ReadFull(conn, reply); err != nil {
		return fmt.Errorf("worker %d receive: %w", worker, err)
	}
	if !bytes.Equal(payload, reply) {
		return fmt.Errorf("worker %d received a different payload", worker)
	}
	return ctx.Err()
}
