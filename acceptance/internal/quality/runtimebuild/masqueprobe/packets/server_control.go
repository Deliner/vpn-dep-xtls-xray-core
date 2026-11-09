package packets

// These controls exercise the real overlaid serverTunnel and writer. Immediate
// consumer mutation/release makes reads after successful send observable to race.
const serverBindingControl = `package masque

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"testing"
	"time"

	"github.com/xtls/xray-core/common/buf"
	masquetransport "github.com/xtls/xray-core/transport/internet/masque"
	wgtun "golang.zx2c4.com/wireguard/tun"
)

func TestNHOPServerQueueBindingCapacityFIFOAndClose(t *testing.T) {
	tunnel := newServerTunnel(nil, nil)
	t.Cleanup(func() {
		for len(tunnel.queue) != 0 {
			(<-tunnel.queue).Release()
		}
	})
	if cap(tunnel.queue) != 512 {
		t.Fatal("observer changed the production capacity")
	}
	for i := range cap(tunnel.queue) {
		packet := buf.FromBytes([]byte{byte(i >> 8), byte(i)})
		if !tunnel.send(packet) {
			packet.Release()
			t.Fatalf("admission refused packet %d below capacity", i)
		}
	}
	nhopRejectServerPacket(t, tunnel)
	tunnel.close()
	nhopRejectServerPacket(t, tunnel)
	for i := range cap(tunnel.queue) {
		packet := <-tunnel.queue
		valid := bytes.Equal(packet.Bytes(), []byte{byte(i >> 8), byte(i)})
		packet.Release()
		if !valid {
			t.Fatalf("queue contents or FIFO changed at packet %d", i)
		}
	}
	fmt.Println("NHOP_SERVER_QUEUE_POLICY limit=512 full=reject closed=reject fifo=true ownership=true")
}

func nhopRejectServerPacket(t *testing.T, tunnel *serverTunnel) {
	t.Helper()
	packet := buf.FromBytes(nhopTestDatagram(4))
	defer packet.Release()
	original := bytes.Clone(packet.Bytes())
	if tunnel.send(packet) || len(tunnel.queue) != cap(tunnel.queue) || !bytes.Equal(packet.Bytes(), original) {
		t.Fatal("rejected packet ownership or queue contents changed")
	}
}

func TestNHOPServerQueueBindingConcurrentOwnership(t *testing.T) {
	tunnel := newServerTunnel(nil, nil)
	consumed := make(chan int, 1)
	go func() {
		count := 0
		for packet := range tunnel.queue {
			clear(packet.Bytes())
			packet.Release()
			count++
		}
		consumed <- count
	}()
	for i := range 256 {
		packet := buf.FromBytes(nhopTestPacket(4))
		if !tunnel.send(packet) {
			packet.Release()
			close(tunnel.queue)
			<-consumed
			t.Fatalf("unexpected admission failure at %d", i)
		}
	}
	close(tunnel.queue)
	if count := <-consumed; count != 256 {
		t.Fatalf("consumer received %d packets", count)
	}
}

func TestNHOPServerQueueBindingWriterAndErrors(t *testing.T) {
	for _, failure := range []error{nil, errors.New("diagnostic write failure")} {
		for _, version := range []int{4, 6} {
			nhopServerWriterControl(t, version, failure)
		}
	}
}

func nhopServerWriterControl(t *testing.T, version int, failure error) {
	t.Helper()
	connection := &nhopQueueConnection{packets: make(chan []byte, 2), failure: failure}
	tunnel := newServerTunnel(connection, nil)
	defer tunnel.close()
	stopped := make(chan struct{})
	go func() {
		new(Server).writeToTunnel(tunnel)
		close(stopped)
	}()
	for _, data := range [][]byte{nhopTestPacket(version), nhopTestDatagram(version)} {
		packet := buf.FromBytes(bytes.Clone(data))
		if !tunnel.send(packet) {
			packet.Release()
			t.Fatal("writer refused a packet")
		}
		select {
		case actual := <-connection.packets:
			if !bytes.Equal(actual, data) {
				t.Fatal("writer changed packet bytes")
			}
		case <-time.After(time.Second):
			t.Fatal("writer did not continue after the previous result")
		}
	}
	tunnel.close()
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("writer did not stop")
	}
}

type nhopQueueConnection struct {
	net.Conn
	packets chan []byte
	failure error
}

func (c *nhopQueueConnection) Write(data []byte) (int, error) {
	c.packets <- bytes.Clone(data)
	if c.failure != nil {
		return 0, c.failure
	}
	return len(data), nil
}

func TestNHOPServerWriteErrorClassification(t *testing.T) {
	tooBig := &masquetransport.PacketTooBigError{ICMP: []byte("private")}
	for _, test := range []struct {
		err error
		want string
	}{
		{nil, "none"}, {tooBig, "packet-too-big"},
		{fmt.Errorf("wrapped: %w", tooBig), "packet-too-big"},
		{net.ErrClosed, "closed"}, {io.ErrClosedPipe, "closed"},
		{context.Canceled, "canceled"}, {context.DeadlineExceeded, "timeout"},
		{&net.OpError{Op: "write", Net: "udp", Err: context.DeadlineExceeded}, "timeout"},
		{errors.New("private"), "other"},
	} {
		if got := nhopServerWriteOutcome(test.err); got != test.want {
			t.Fatalf("error class = %q, want %q", got, test.want)
		}
		packet := nhopTestDatagram(4)
		nhopTraceServerWrite(nhopServerQueueSnapshot(&packet, &packet, packet), 0, 512, test.err)
	}
}

func TestNHOPServerQueueBindingPacketTooBig(t *testing.T) {
	icmp := []byte{1, 2, 3, 4}
	failure := &masquetransport.PacketTooBigError{ICMP: bytes.Clone(icmp)}
	connection := &nhopQueueConnection{packets: make(chan []byte, 1), failure: failure}
	tunnel := newServerTunnel(connection, nil)
	defer tunnel.close()
	device := &nhopICMPDevice{packets: make(chan []byte, 1)}
	server, stopped := &Server{dev: device}, make(chan struct{})
	go func() {
		server.writeToTunnel(tunnel)
		close(stopped)
	}()
	packet := buf.FromBytes(nhopTestPacket(4))
	if !tunnel.send(packet) {
		packet.Release()
		t.Fatal("writer refused a packet")
	}
	select {
	case actual := <-device.packets:
		if !bytes.Equal(actual, icmp) || !bytes.Equal(failure.ICMP, icmp) {
			t.Fatal("observer changed the ICMP path or error ownership")
		}
	case <-time.After(time.Second):
		t.Fatal("writer did not return ICMP to the device")
	}
	tunnel.close()
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("writer did not stop")
	}
}

type nhopICMPDevice struct {
	wgtun.Device
	packets chan []byte
}

func (d *nhopICMPDevice) Write(packets [][]byte, offset int) (int, error) {
	for _, packet := range packets {
		d.packets <- bytes.Clone(packet[offset:])
	}
	return len(packets), nil
}
`
