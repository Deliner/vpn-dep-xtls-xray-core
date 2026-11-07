package ipcheck

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"slices"
	"strconv"
	"testing"
	"time"
)

func probeSOCKS(t *testing.T, target string) (int, <-chan string) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	observed, done := make(chan string, 1), make(chan struct{})
	go func() {
		defer close(done)
		connection, err := listener.Accept()
		if err != nil {
			return
		}
		defer connection.Close()
		stop := context.AfterFunc(ctx, func() { connection.Close() })
		defer stop()
		connection.SetDeadline(time.Now().Add(5 * time.Second))
		serveProbeSOCKS(ctx, connection, target, observed)
	}()
	t.Cleanup(func() {
		cancel()
		listener.Close()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Error("SOCKS fixture did not stop")
		}
	})
	return listener.Addr().(*net.TCPAddr).Port, observed
}

func probeSOCKSRequest(connection net.Conn) (string, error) {
	header := make([]byte, 2)
	if _, err := io.ReadFull(connection, header); err != nil {
		return "", err
	}
	if header[0] != 5 || header[1] == 0 {
		return "", fmt.Errorf("invalid SOCKS greeting")
	}
	methods := make([]byte, int(header[1]))
	if _, err := io.ReadFull(connection, methods); err != nil || !slices.Contains(methods, byte(0)) {
		return "", fmt.Errorf("missing unauthenticated SOCKS method")
	}
	if _, err := connection.Write([]byte{5, 0}); err != nil {
		return "", err
	}
	header = make([]byte, 4)
	if _, err := io.ReadFull(connection, header); err != nil {
		return "", err
	}
	if header[0] != 5 || header[1] != 1 || header[2] != 0 {
		return "", fmt.Errorf("expected SOCKS CONNECT")
	}
	host, err := probeSOCKSHost(connection, header[3])
	if err != nil {
		return "", err
	}
	port := make([]byte, 2)
	if _, err := io.ReadFull(connection, port); err != nil {
		return "", err
	}
	return net.JoinHostPort(host, strconv.Itoa(int(binary.BigEndian.Uint16(port)))), nil
}

func probeSOCKSHost(reader io.Reader, kind byte) (string, error) {
	length := 4
	switch kind {
	case 1:
	case 4:
		length = 16
	case 3:
		value := make([]byte, 1)
		if _, err := io.ReadFull(reader, value); err != nil {
			return "", err
		}
		length = int(value[0])
	default:
		return "", fmt.Errorf("invalid SOCKS address kind")
	}
	value := make([]byte, length)
	if _, err := io.ReadFull(reader, value); err != nil {
		return "", err
	}
	if kind == 3 {
		return string(value), nil
	}
	return net.IP(value).String(), nil
}

func serveProbeSOCKS(ctx context.Context, connection net.Conn, target string, observed chan<- string) {
	destination, err := probeSOCKSRequest(connection)
	if err != nil {
		return
	}
	observed <- destination
	dialer := net.Dialer{Timeout: time.Second}
	upstream, err := dialer.DialContext(ctx, "tcp", target)
	if err != nil {
		return
	}
	defer upstream.Close()
	stop := context.AfterFunc(ctx, func() { upstream.Close() })
	defer stop()
	if _, err := connection.Write([]byte{5, 0, 0, 1, 127, 0, 0, 1, 0, 0}); err != nil {
		return
	}
	done := make(chan struct{})
	go func() {
		io.Copy(upstream, connection)
		upstream.Close()
		close(done)
	}()
	io.Copy(connection, upstream)
	connection.Close()
	<-done
}
