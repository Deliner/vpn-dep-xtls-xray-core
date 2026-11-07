package traffic

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"strconv"
	"time"
)

func openSession(ctx context.Context, endpoint Endpoint, target int) (*session, error) {
	tcp, _, err := socksOpen(ctx, endpoint.Port, target, 1)
	if err != nil {
		return nil, fmt.Errorf("TCP CONNECT: %w", err)
	}
	control, port, err := socksOpen(ctx, endpoint.Port, 0, 3)
	if err != nil {
		tcp.Close()
		return nil, fmt.Errorf("UDP ASSOCIATE: %w", err)
	}
	dialer := net.Dialer{}
	udp, err := dialer.DialContext(ctx, "udp4", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		tcp.Close()
		control.Close()
		return nil, err
	}
	s := &session{tcp: tcp, control: control, udp: udp}
	s.stop = context.AfterFunc(ctx, s.closeConnections)
	return s, nil
}

func (s *session) closeConnections() {
	s.tcp.Close()
	s.control.Close()
	s.udp.Close()
}

func (s *session) close() {
	s.stop()
	s.closeConnections()
}

func (s *session) exchange(id string, sequence, target int, result *Result) error {
	tcp, udp := payload(id, sequence, tcpBytes), payload(id, sequence, udpBytes)
	s.tcp.SetDeadline(time.Now().Add(10 * time.Second))
	s.udp.SetDeadline(time.Now().Add(10 * time.Second))
	if err := writeFrame(s.tcp, tcp); err != nil {
		return fmt.Errorf("TCP send: %w", err)
	}
	header := []byte{0, 0, 0, 1, 198, 18, 0, 2, byte(target >> 8), byte(target)}
	if _, err := s.udp.Write(append(header, udp...)); err != nil {
		return fmt.Errorf("UDP send: %w", err)
	}
	got, err := readFrame(s.tcp)
	if err != nil {
		return fmt.Errorf("TCP receive: %w", err)
	}
	if !bytes.Equal(got, tcp) {
		return fmt.Errorf("TCP payload differs for %s round %d", id, sequence)
	}
	result.TCPRounds++
	if err := s.receiveUDP(udp); err != nil {
		return err
	}
	result.UDPRounds++
	return nil
}

func (s *session) receiveUDP(expected []byte) error {
	buffer := make([]byte, 65535)
	size, err := s.udp.Read(buffer)
	if err != nil {
		return fmt.Errorf("UDP receive: %w", err)
	}
	if size < 10 || !bytes.Equal(buffer[:4], []byte{0, 0, 0, 1}) {
		return fmt.Errorf("invalid SOCKS UDP response")
	}
	if !bytes.Equal(buffer[10:size], expected) {
		return fmt.Errorf("UDP payload differs (route/round/data)")
	}
	return nil
}
