package traffic

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"strconv"
	"time"
)

// OpenUDPProbe keeps one SOCKS association and one UDP source socket alive across
// destination changes and rejected packets. The SOCKS peer is IPv4 loopback.
func OpenUDPProbe(ctx context.Context, port int) (*UDPProbe, error) {
	control, bound, err := socksOpen(ctx, port, 0, 3)
	if err != nil {
		return nil, err
	}
	dialer := net.Dialer{}
	connection, err := dialer.DialContext(ctx, "udp4", net.JoinHostPort("127.0.0.1", strconv.Itoa(bound)))
	if err != nil {
		control.Close()
		return nil, err
	}
	probe := &UDPProbe{control: control, connection: connection}
	probe.stop = context.AfterFunc(ctx, probe.closeConnections)
	return probe, nil
}

func (p *UDPProbe) closeConnections() {
	p.connection.Close()
	p.control.Close()
}

func (p *UDPProbe) Close() {
	p.stop()
	p.closeConnections()
}

func (p *UDPProbe) Exchange(target int, data []byte, timeout time.Duration, address ...[4]byte) ([]byte, error) {
	if err := p.connection.SetDeadline(time.Now().Add(timeout)); err != nil {
		return nil, err
	}
	ip := [4]byte{127, 0, 0, 1}
	if len(address) != 0 {
		ip = address[0]
	}
	header := []byte{0, 0, 0, 1, ip[0], ip[1], ip[2], ip[3], byte(target >> 8), byte(target)}
	if _, err := p.connection.Write(append(header, data...)); err != nil {
		return nil, err
	}
	buffer := make([]byte, 65535)
	size, err := p.connection.Read(buffer)
	if err != nil {
		return nil, err
	}
	if size < len(header) || !bytes.Equal(buffer[:len(header)], header) {
		return nil, fmt.Errorf("SOCKS UDP reply source=%x, want=%x", buffer[:min(size, len(header))], header)
	}
	return buffer[len(header):size], nil
}
