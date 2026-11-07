package controller

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"vpn-script/internal/control/client/session"
)

func testSocket(t *testing.T) (*socket, string) {
	t.Helper()
	// Short direct temp name fits sockaddr_un on macOS without touching host IP.
	directory, err := os.MkdirTemp("", "nhop-ipc-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(directory) })
	path := filepath.Join(directory, "c")
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	server := accepting(listener)
	t.Cleanup(server.close)
	return server, path
}

func wire(path, request string) (Diagnostic, error) {
	connection, err := net.DialUnix("unix", nil, &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		return Diagnostic{}, err
	}
	defer connection.Close()
	connection.SetDeadline(time.Now().Add(5 * time.Second))
	if _, err := fmt.Fprint(connection, request); err != nil {
		return Diagnostic{}, err
	}
	if err := connection.CloseWrite(); err != nil {
		return Diagnostic{}, err
	}
	var result Diagnostic
	err = json.NewDecoder(connection).Decode(&result)
	return result, err
}

func TestMalformedControlNeverReachesSession(t *testing.T) {
	server, path := testSocket(t)
	for _, value := range []string{`{"operation":"stop","extra":true}`, `{"operation":"stop"}{}`, `{"operation":"flush"}`} {
		result, err := wire(path, value)
		if err != nil || result.Error == "" {
			t.Fatal("invalid command was not rejected", result, err)
		}
	}
	select {
	case c := <-server.calls:
		t.Fatal("invalid request was delivered", c.operation)
	default:
	}
}

func TestConcurrentControlRequestsReceiveCompleteReplies(t *testing.T) {
	server, path := testSocket(t)
	var count atomic.Int32
	stop, stopped := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(stopped)
		for {
			select {
			case c := <-server.calls:
				count.Add(1)
				c.reply <- Diagnostic{Controller: "active", Status: session.Status{Phase: "running"}}
			case <-stop:
				return
			}
		}
	}()
	defer func() {
		close(stop)
		<-stopped
	}()
	var workers sync.WaitGroup
	for range 12 {
		workers.Go(func() {
			result, err := wire(path, `{"operation":"diagnostics"}`)
			if err != nil || result.Controller != "active" || result.Status.Phase != "running" {
				t.Error("incomplete concurrent response", result, err)
			}
		})
	}
	workers.Wait()
	if count.Load() != 12 {
		t.Fatal("requests were lost or delivered twice", count.Load())
	}
}
