package connectip

import (
	"bytes"
	"errors"
	"io"
	"net/netip"
	"testing"
	"time"

	"github.com/apernet/quic-go"
	"github.com/apernet/quic-go/quicvarint"
)

func TestNHOPHTTP3CapsulesAndDatagrams(t *testing.T) {
	client, server := setupConns(t)
	for _, conn := range []*Conn{client, server} {
		conn.SetAssignedAddresses([]netip.Prefix{
			netip.PrefixFrom(testDst4, 32), netip.PrefixFrom(testDst6, 128),
		})
	}
	for _, pair := range []struct {
		name string
		from *Conn
		to   *Conn
	}{{"client-to-server", client, server}, {"server-to-client", server, client}} {
		t.Run(pair.name, func(t *testing.T) {
			checkHTTP3CapsuleSequence(t, pair.from, pair.to)
		})
	}
}

func checkHTTP3CapsuleSequence(t *testing.T, from, to *Conn) {
	t.Helper()
	for _, packet := range [][]byte{
		ipv4Packet(64, 17, testSrc4, testDst4, nil, []byte("before-capsule")),
		ipv6Packet(64, 17, testSrc6, testDst6, bytes.Repeat([]byte{0xab}, 1240)),
		ipv4Packet(64, 17, testSrc4, testDst4, nil, bytes.Repeat([]byte{0xcd}, 65515)),
		ipv6Packet(64, 17, testSrc6, testDst6, []byte("after-capsule")),
	} {
		data := append([]byte{0}, packet...)
		if len(packet) > 1350 {
			var tooLarge *quic.DatagramTooLargeError
			if err := from.h3.SendDatagram(data); !errors.As(err, &tooLarge) {
				t.Fatalf("oversized packet did not exceed the QUIC DATAGRAM limit: %v", err)
			}
		}
		if len(packet) > 1200 {
			if err := from.write(encodeTestDatagramCapsule(data)); err != nil {
				t.Fatal(err)
			}
		} else if err := from.h3.SendDatagram(data); err != nil {
			t.Fatal(err)
		}
		if actual := readTestIPPacket(t, to); !bytes.Equal(actual, packet) {
			t.Fatalf("packet changed: got %d bytes, want %d", len(actual), len(packet))
		}
	}
}

func encodeTestDatagramCapsule(data []byte) []byte {
	encoded := quicvarint.Append(nil, uint64(capsuleTypeDatagram))
	encoded = quicvarint.Append(encoded, uint64(len(data)))
	return append(encoded, data...)
}

func readTestIPPacket(t *testing.T, conn *Conn) []byte {
	t.Helper()
	type result struct {
		packet []byte
		err    error
	}
	finished := make(chan result, 1)
	go func() {
		packet := make([]byte, maxCapsulePacketSize)
		n, err := conn.ReadPacket(packet)
		finished <- result{packet: packet[:n], err: err}
	}()
	select {
	case actual := <-finished:
		if actual.err != nil {
			t.Fatal(actual.err)
		}
		return actual.packet
	case <-time.After(time.Second):
		t.Fatal("HTTP/3 DATAGRAM capsule was not delivered")
		return nil
	}
}

func TestNHOPHTTP3CapsuleValidation(t *testing.T) {
	client, server := setupConns(t)
	server.SetAssignedAddresses([]netip.Prefix{netip.PrefixFrom(testDst4, 32)})
	if err := server.AssignAddresses([]netip.Prefix{netip.PrefixFrom(testSrc4, 32)}); err != nil {
		t.Fatal(err)
	}
	valid := ipv4Packet(64, 17, testSrc4, testDst4, nil, []byte("allowed"))
	for _, data := range [][]byte{
		append([]byte{7}, valid...),
		{0, 0x45},
		append([]byte{0}, ipv4Packet(64, 17, netip.MustParseAddr("192.0.2.2"), testDst4, nil, nil)...),
		append([]byte{0}, ipv4Packet(64, 17, testSrc4, testSrc4, nil, nil)...),
		make([]byte, maxCapsulePacketSize+2),
		append([]byte{0}, valid...),
	} {
		if err := client.write(encodeTestDatagramCapsule(data)); err != nil {
			t.Fatal(err)
		}
	}
	if packet := readTestIPPacket(t, server); !bytes.Equal(packet, valid) {
		t.Fatalf("invalid context, source or destination accepted: %x", packet)
	}
}

func TestNHOPHTTP3TruncatedCapsuleClosesRequest(t *testing.T) {
	client, server := setupConns(t)
	truncated := quicvarint.Append(nil, uint64(capsuleTypeDatagram))
	truncated = quicvarint.Append(truncated, 10)
	if err := client.write(append(truncated, 0, 0x45)); err != nil {
		t.Fatal(err)
	}
	if err := client.str.Close(); err != nil {
		t.Fatal(err)
	}
	finished := make(chan error, 1)
	go func() {
		_, err := server.ReadPacket(make([]byte, 1500))
		finished <- err
	}()
	select {
	case err := <-finished:
		var closed *CloseError
		if !errors.As(err, &closed) || !closed.Remote {
			t.Fatalf("truncated capsule did not close its request: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("truncated capsule left ReadPacket blocked")
	}
	select {
	case <-server.h3Receiver.done:
	case <-time.After(time.Second):
		t.Fatal("closed request retained its datagram reader")
	}
}

func TestNHOPHTTP3CapsuleShortBuffer(t *testing.T) {
	client, server := setupConns(t)
	server.SetAssignedAddresses([]netip.Prefix{netip.PrefixFrom(testDst4, 32)})
	packet := ipv4Packet(64, 17, testSrc4, testDst4, nil, []byte("large-for-reader"))
	if err := client.write(encodeTestDatagramCapsule(append([]byte{0}, packet...))); err != nil {
		t.Fatal(err)
	}
	finished := make(chan error, 1)
	go func() {
		_, err := server.ReadPacket(make([]byte, 10))
		finished <- err
	}()
	select {
	case err := <-finished:
		if !errors.Is(err, io.ErrShortBuffer) {
			t.Fatalf("short buffer accepted: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("short-buffer ReadPacket blocked")
	}
	if err := client.h3.SendDatagram(append([]byte{0}, packet...)); err != nil {
		t.Fatal(err)
	}
	if actual := readTestIPPacket(t, server); !bytes.Equal(actual, packet) {
		t.Fatal("datagrams after short-buffer failure changed")
	}
}
