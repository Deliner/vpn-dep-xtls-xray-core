package traffic

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"time"
)

var ErrUDPAllocation = errors.New("SOCKS UDP allocation failed")

// AssociationPorts keeps every control connection alive while requesting more
// UDP listeners. Explicit TCP source ports avoid the intentionally small UDP range.
func AssociationPorts(ctx context.Context, socksPort, sourcePort, count int) ([]int, error) {
	var connections []net.Conn
	var ports []int
	defer func() {
		for _, connection := range connections {
			connection.Close()
		}
	}()
	for index := range count {
		dialer := net.Dialer{Timeout: time.Second, LocalAddr: &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: sourcePort + index}}
		connection, err := dialer.DialContext(ctx, "tcp4", net.JoinHostPort("127.0.0.1", strconv.Itoa(socksPort)))
		if err != nil {
			return ports, fmt.Errorf("TCP control connection: %w", err)
		}
		connections = append(connections, connection)
		connection.SetDeadline(time.Now().Add(time.Second))
		port, err := socksHandshake(connection, 0, 3)
		if err != nil {
			return ports, fmt.Errorf("%w: %v", ErrUDPAllocation, err)
		}
		ports = append(ports, port)
	}
	return ports, nil
}
