package masqueprobe

import "strings"

func packetControl(capsules bool) string {
	if !capsules {
		return packetBindingControl
	}
	return strings.NewReplacer(
		"closeChan: make(chan struct{}), localRoutes:", "h3Receiver: newDatagramReceiver(stream), closeChan: make(chan struct{}), localRoutes:",
		"packet := nhopTestPacket(4)", "defer conn.h3Receiver.stop()\n\tpacket := nhopTestPacket(4)",
	).Replace(packetBindingControl) + packetCapsuleControl
}

func deviceControl(fair bool) string {
	if !fair {
		return deviceBindingControl
	}
	return strings.NewReplacer(
		"github.com/xtls/xray-core/common/buf", "github.com/xtls/xray-core/proxy/wireguard/packetqueue",
		"make(chan *buf.Buffer, netTunQueue)", "packetqueue.New(netTunQueue)",
		"make(chan *buf.Buffer)", "packetqueue.New(1)",
		"len(device.packets)", "device.packets.Len()",
		" || cap(device.packets) != netTunQueue", "",
		"close(device.closed)", "close(device.closed)\n\tdevice.packets.Close()",
	).Replace(deviceBindingControl)
}

// These controls run against the actual overlaid CONNECT-IP Conn. They exercise
// success and failure boundaries without network access. The capsule-capable
// implementation owns a lazy datagram reader, which the control stops explicitly.
const packetBindingControl = `package connectip

import (
	"bytes"
	"errors"
	"io"
	"net/netip"
	"testing"
)

func TestNHOPPacketBindingPreservesDelivery(t *testing.T) {
	datagrams := make(chan []byte, 2)
	stream := &mockStream{datagrams: datagrams}
	conn := &Conn{h3: stream, closeChan: make(chan struct{}), localRoutes: []IPRoute{{
		StartIP: netip.IPv4Unspecified(), EndIP: netip.MustParseAddr("255.255.255.255"),
	}}}
	packet := nhopTestPacket(4)
	icmp, err := conn.WritePacket(packet)
	if err != nil || len(icmp) != 0 || len(stream.sent) != 1 {
		t.Fatalf("write behavior changed: icmp=%x err=%v sends=%d", icmp, err, len(stream.sent))
	}
	if !bytes.Equal(stream.sent[0], append([]byte{0}, packet...)) {
		t.Fatal("observer changed datagram or context ID")
	}
	datagrams <- stream.sent[0]
	datagrams <- stream.sent[0]
	if n, err := conn.ReadPacket(make([]byte, 1)); n != 0 || !errors.Is(err, io.ErrShortBuffer) {
		t.Fatalf("short receive behavior changed: n=%d err=%v", n, err)
	}
	buffer := make([]byte, len(packet))
	n, err := conn.ReadPacket(buffer)
	if err != nil || n != len(packet) || !bytes.Equal(buffer, packet) {
		t.Fatalf("read behavior changed: n=%d err=%v", n, err)
	}
}

func TestNHOPPacketBindingPreservesSendFailure(t *testing.T) {
	failure := errors.New("diagnostic sentinel")
	stream := &mockStream{sendDatagramErr: failure}
	conn := &Conn{h3: stream, closeChan: make(chan struct{})}
	icmp, err := conn.WritePacket(nhopTestPacket(4))
	if len(icmp) != 0 || !errors.Is(err, failure) || len(stream.sent) != 0 {
		t.Fatalf("send failure changed: icmp=%x err=%v sends=%d", icmp, err, len(stream.sent))
	}
	conn.closeErr = failure
	close(conn.closeChan)
	if _, err := conn.WritePacket(nhopTestPacket(4)); !errors.Is(err, failure) {
		t.Fatalf("closed write behavior changed: %v", err)
	}
	if _, err := conn.ReadPacket(make([]byte, 1500)); !errors.Is(err, failure) {
		t.Fatalf("closed read behavior changed: %v", err)
	}
}
`

const packetCapsuleControl = `
func TestNHOPPacketBindingPreservesCapsuleDelivery(t *testing.T) {
	stream := &mockStream{}
	reader := newDatagramReceiver(stream)
	defer reader.stop()
	datagrams := make(chan []byte, 1)
	conn := &Conn{h3: stream, h3Receiver: reader, datagrams: datagrams, closeChan: make(chan struct{}),
		localRoutes: []IPRoute{{StartIP: netip.IPv4Unspecified(), EndIP: netip.MustParseAddr("255.255.255.255")}}}
	packet := nhopTestPacket(4)
	datagrams <- append([]byte{0}, packet...)
	buffer := make([]byte, len(packet))
	n, err := conn.ReadPacket(buffer)
	if err != nil || n != len(packet) || !bytes.Equal(buffer, packet) {
		t.Fatalf("capsule delivery changed: n=%d err=%v", n, err)
	}
}
`

// Offer is observed while the producer owns the copied buffer; read is observed
// before release by the consumer. No observer accesses a buffer after transfer.
const deviceBindingControl = `package wireguard

import (
	"bytes"
	"errors"
	"os"
	"testing"

	"github.com/xtls/xray-core/common/buf"
)

func TestNHOPDeviceBindingPreservesFIFOAndCopy(t *testing.T) {
	device := &netTun{packets: make(chan *buf.Buffer, netTunQueue), closed: make(chan struct{})}
	first, second := nhopTestPacket(4), nhopTestPacket(4)
	second[24] ^= 1
	for _, packet := range [][]byte{first, second} {
		borrowed := bytes.Clone(packet)
		if err := device.WritePacket(borrowed[:20], borrowed[20:]); err != nil {
			t.Fatal(err)
		}
		clear(borrowed)
	}
	for _, expected := range [][]byte{first, second} {
		buffer, sizes := make([]byte, len(expected)+3), []int{0}
		n, err := device.Read([][]byte{buffer}, sizes, 3)
		if err != nil || n != 1 || sizes[0] != len(expected) || !bytes.Equal(buffer[3:], expected) {
			t.Fatalf("device ownership, offset or FIFO changed: n=%d size=%d err=%v", n, sizes[0], err)
		}
	}
	if len(device.packets) != 0 || cap(device.packets) != netTunQueue {
		t.Fatal("observer changed device admission or queue capacity")
	}
}

func TestNHOPDeviceBindingPreservesClosedState(t *testing.T) {
	device := &netTun{packets: make(chan *buf.Buffer), closed: make(chan struct{})}
	close(device.closed)
	packet := nhopTestPacket(4)
	if err := device.WritePacket(packet[:20], packet[20:]); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("closed write behavior changed: %v", err)
	}
	if n, err := device.Read([][]byte{make([]byte, 1500)}, []int{0}, 0); n != 0 || !errors.Is(err, os.ErrClosed) {
		t.Fatalf("closed read behavior changed: n=%d err=%v", n, err)
	}
}
`
