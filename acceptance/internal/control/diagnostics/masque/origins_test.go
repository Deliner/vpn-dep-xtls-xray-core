package masque

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestMASQUEOriginHelper(t *testing.T) {
	if os.Getenv("NHOP_MASQUE_ORIGIN") != "1" {
		t.Skip("isolated subprocess origin")
	}
	requireIsolation(t)
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	origin := openOrigins(t)
	defer origin.close(t)
	go serveTCP(ctx, origin.tcp)
	go func() {
		origin.done <- serveUDP(origin.udp, origin.log)
		close(origin.done)
	}()
	if err := os.WriteFile(filepath.Join(os.Getenv("NHOP_MASQUE_REPORT"), "origin-ready"), []byte("ready\n"), 0600); err != nil {
		t.Fatal(err)
	}
	select {
	case <-ctx.Done():
	case err := <-origin.done:
		t.Fatal("UDP origin stopped unexpectedly", err)
	}
}

func openOrigins(t *testing.T) *origins {
	t.Helper()
	tcp, err := net.Listen("tcp", tcpOrigin)
	if err != nil {
		t.Fatal(err)
	}
	udp, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.ParseIP("198.18.0.2"), Port: udpOriginPort})
	if err != nil {
		tcp.Close()
		t.Fatal(err)
	}
	if err := prepareOriginUDP(t, udp); err != nil {
		tcp.Close()
		udp.Close()
		t.Fatal(err)
	}
	log, err := os.OpenFile(filepath.Join(os.Getenv("NHOP_MASQUE_REPORT"), "origin-udp.log"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		tcp.Close()
		udp.Close()
		t.Fatal(err)
	}
	return &origins{tcp: tcp, udp: udp, log: log, done: make(chan error, 1)}
}

func (o *origins) close(t *testing.T) {
	t.Helper()
	o.tcp.Close()
	o.udp.Close()
	if err := <-o.done; err != nil {
		t.Error(err)
	}
	if err := o.log.Close(); err != nil {
		t.Error(err)
	}
}

func serveTCP(ctx context.Context, listener net.Listener) {
	for {
		connection, err := listener.Accept()
		if err != nil {
			return
		}
		go func() {
			defer connection.Close()
			stop := context.AfterFunc(ctx, func() { connection.Close() })
			defer stop()
			_, _ = io.Copy(connection, connection)
		}()
	}
}

func serveUDP(connection net.PacketConn, log io.Writer) error {
	var buffer [65535]byte
	for {
		size, peer, err := connection.ReadFrom(buffer[:])
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			return err
		}
		if _, err := fmt.Fprintf(log, "%s id=%x bytes=%d\n", time.Now().UTC().Format(time.RFC3339Nano), buffer[:min(size, 16)], size); err != nil {
			return err
		}
		if _, err := connection.WriteTo(buffer[:size], peer); err != nil {
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			return err
		}
	}
}

func startOrigins(t *testing.T, directory string) {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	child := startManaged(t, directory, "origin", exitNamespace, "env", "NHOP_MASQUE_ORIGIN=1", executable, "-test.run=^TestMASQUEOriginHelper$")
	child.cleanExit = true
	awaitFile(t, child, filepath.Join(directory, "origin-ready"))
}
