package traffic

import (
	"bytes"
	"fmt"
	"net"
	"strconv"
	"time"
)

// UDPRoundTrip is the bounded single-datagram probe used by payload and failure tests.
func UDPRoundTrip(port, target int, data []byte, address ...[4]byte) ([]byte, error) {
	control, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), time.Second)
	if err != nil {
		return nil, err
	}
	defer control.Close()
	if err := control.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
		return nil, err
	}
	association, err := socksHandshake(control, 0, 3)
	if err != nil {
		return nil, err
	}
	return udpPacket(association, target, data, address...)
}

func udpPacket(port, target int, data []byte, address ...[4]byte) ([]byte, error) {
	connection, err := net.DialUDP("udp4", nil, &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: port})
	if err != nil {
		return nil, err
	}
	defer connection.Close()
	if err := connection.SetDeadline(time.Now().Add(2 * time.Second)); err != nil {
		return nil, err
	}
	ip := [4]byte{127, 0, 0, 1}
	if len(address) != 0 {
		ip = address[0]
	}
	packet := []byte{0, 0, 0, 1, ip[0], ip[1], ip[2], ip[3], byte(target >> 8), byte(target)}
	if _, err := connection.Write(append(packet, data...)); err != nil {
		return nil, err
	}
	buffer := make([]byte, 65535)
	length, err := connection.Read(buffer)
	if err != nil {
		return nil, err
	}
	if length < 10 || !bytes.Equal(buffer[:4], []byte{0, 0, 0, 1}) {
		return nil, fmt.Errorf("invalid SOCKS UDP reply")
	}
	return buffer[10:length], nil
}
