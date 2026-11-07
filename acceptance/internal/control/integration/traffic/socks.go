package traffic

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strconv"
	"time"
)

func socksOpen(ctx context.Context, port, target int, command byte) (net.Conn, int, error) {
	dialer := net.Dialer{Timeout: 5 * time.Second}
	connection, err := dialer.DialContext(ctx, "tcp4", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		return nil, 0, err
	}
	stop := context.AfterFunc(ctx, func() { connection.Close() })
	defer stop()
	connection.SetDeadline(time.Now().Add(10 * time.Second))
	bound, err := socksHandshake(connection, target, command)
	if err != nil {
		connection.Close()
		return nil, 0, err
	}
	return connection, bound, nil
}

func socksHandshake(connection net.Conn, target int, command byte) (int, error) {
	if _, err := connection.Write([]byte{5, 1, 0}); err != nil {
		return 0, err
	}
	answer := make([]byte, 2)
	if _, err := io.ReadFull(connection, answer); err != nil {
		return 0, err
	}
	if answer[0] != 5 || answer[1] != 0 {
		return 0, fmt.Errorf("SOCKS negotiation rejected")
	}
	request := []byte{5, command, 0, 1, 198, 18, 0, 2, byte(target >> 8), byte(target)}
	if command == 3 {
		clear(request[4:])
	}
	if _, err := connection.Write(request); err != nil {
		return 0, err
	}
	return socksReply(connection)
}

func socksReply(reader io.Reader) (int, error) {
	header := make([]byte, 4)
	if _, err := io.ReadFull(reader, header); err != nil {
		return 0, err
	}
	if header[0] != 5 || header[1] != 0 || header[2] != 0 {
		return 0, fmt.Errorf("SOCKS request rejected: %v", header)
	}
	size := 4
	if header[3] == 4 {
		size = 16
	} else if header[3] != 1 {
		return 0, fmt.Errorf("SOCKS lab requires IP-literal replies")
	}
	address := make([]byte, size+2)
	_, err := io.ReadFull(reader, address)
	return int(binary.BigEndian.Uint16(address[size:])), err
}
