package network

import (
	"io"
	"net"
	"sync/atomic"
	"testing"
)

func kernelTCP(t *testing.T, address string) *atomic.Int64 {
	t.Helper()
	listener, err := net.Listen("tcp", address)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { listener.Close() })
	count := new(atomic.Int64)
	go func() {
		for {
			connection, err := listener.Accept()
			if err != nil {
				return
			}
			go echoKernelTCP(connection, count)
		}
	}()
	return count
}

func echoKernelTCP(connection net.Conn, count *atomic.Int64) {
	defer connection.Close()
	var buffer [1]byte
	for {
		if _, err := io.ReadFull(connection, buffer[:]); err != nil {
			return
		}
		count.Add(1)
		if _, err := connection.Write(buffer[:]); err != nil {
			return
		}
	}
}

func kernelUDP(t *testing.T, address string) *atomic.Int64 {
	t.Helper()
	connection, err := net.ListenPacket("udp", address)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { connection.Close() })
	count := new(atomic.Int64)
	go func() {
		var buffer [1]byte
		for {
			_, source, err := connection.ReadFrom(buffer[:])
			if err != nil {
				return
			}
			count.Add(1)
			connection.WriteTo(buffer[:], source)
		}
	}()
	return count
}
