package traffic

import (
	"bytes"
	"net"
	"testing"
	"time"
)

func TestUDPProbePreservesDatagramOrderAndDuplicates(t *testing.T) {
	probe, relay := probePair(t)
	deadline := time.Now().Add(time.Second)
	var requests [][]byte
	var peer net.Addr
	for _, payload := range []string{"first", "second"} {
		if err := probe.Send(18081, []byte(payload), deadline, [4]byte{198, 18, 0, 2}); err != nil {
			t.Fatal(err)
		}
		request, sender := probeRequest(t, relay)
		if !bytes.Equal(request[:10], []byte{0, 0, 0, 1, 198, 18, 0, 2, 0x46, 0xa1}) {
			t.Fatalf("unexpected SOCKS UDP destination: %x", request)
		}
		requests, peer = append(requests, request), sender
	}
	for _, index := range []int{1, 0, 0} {
		if _, err := relay.WriteTo(requests[index], peer); err != nil {
			t.Fatal(err)
		}
	}
	for _, want := range []string{"second", "first", "first"} {
		got, err := probe.Receive(18081, deadline, [4]byte{198, 18, 0, 2})
		if err != nil || string(got) != want {
			t.Fatalf("received %q / %v, want %q", got, err, want)
		}
	}
}

func TestUDPProbeRejectsForeignSourceWithoutDiscardingAssociation(t *testing.T) {
	probe, relay := probePair(t)
	deadline := time.Now().Add(time.Second)
	if err := probe.Send(18081, []byte("expected"), deadline); err != nil {
		t.Fatal(err)
	}
	request, peer := probeRequest(t, relay)
	foreign := bytes.Clone(request)
	foreign[7] = 2
	for _, packet := range [][]byte{{0}, foreign, request} {
		if _, err := relay.WriteTo(packet, peer); err != nil {
			t.Fatal(err)
		}
	}
	for range 2 {
		if _, err := probe.Receive(18081, deadline); err == nil {
			t.Fatal("truncated or foreign-source response was accepted")
		}
	}
	got, err := probe.Receive(18081, deadline)
	if err != nil || string(got) != "expected" {
		t.Fatalf("association did not recover: %q / %v", got, err)
	}
}

func TestUDPProbeHonorsAbsoluteReadDeadline(t *testing.T) {
	probe, _ := probePair(t)
	_, err := probe.Receive(18081, time.Now().Add(-time.Second))
	timeout, ok := err.(net.Error)
	if !ok || !timeout.Timeout() {
		t.Fatalf("expired read deadline did not terminate: %v", err)
	}
}

func probePair(t *testing.T) (*UDPProbe, net.PacketConn) {
	t.Helper()
	relay, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { relay.Close() })
	if err := relay.SetDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	connection, err := net.Dial("udp4", relay.LocalAddr().String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { connection.Close() })
	return &UDPProbe{connection: connection}, relay
}

func probeRequest(t *testing.T, relay net.PacketConn) ([]byte, net.Addr) {
	t.Helper()
	data := make([]byte, 1024)
	size, peer, err := relay.ReadFrom(data)
	if err != nil || size < 10 {
		t.Fatalf("invalid probe request: bytes=%d / %v", size, err)
	}
	return data[:size], peer
}
