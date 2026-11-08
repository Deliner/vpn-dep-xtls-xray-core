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
	deadline := time.Now().Add(timeout)
	if err := p.Send(target, data, deadline, address...); err != nil {
		return nil, err
	}
	return p.Receive(target, deadline, address...)
}

// Send and Receive separate transmission from observation for controlled bursts.
// The caller owns correlation and the absolute deadline; the association stays open.
func (p *UDPProbe) Send(target int, data []byte, deadline time.Time, address ...[4]byte) error {
	if err := p.connection.SetWriteDeadline(deadline); err != nil {
		return err
	}
	header := udpProbeHeader(target, address...)
	_, err := p.connection.Write(append(header, data...))
	return err
}

func (p *UDPProbe) Receive(target int, deadline time.Time, address ...[4]byte) ([]byte, error) {
	if err := p.connection.SetReadDeadline(deadline); err != nil {
		return nil, err
	}
	header := udpProbeHeader(target, address...)
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

func udpProbeHeader(target int, address ...[4]byte) []byte {
	ip := [4]byte{127, 0, 0, 1}
	if len(address) != 0 {
		ip = address[0]
	}
	return []byte{0, 0, 0, 1, ip[0], ip[1], ip[2], ip[3], byte(target >> 8), byte(target)}
}
