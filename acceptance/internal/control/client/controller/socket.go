package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"time"

	"vpn-script/internal/control/client/host"
)

type socket struct {
	listener *net.UnixListener
	calls    chan call
	cancel   context.CancelFunc
	finished chan struct{}
}

func listen(owner *host.Owner) (*socket, error) {
	if err := owner.PrepareSocket(); err != nil {
		return nil, err
	}
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: host.Socket, Net: "unix"})
	if err != nil {
		return nil, err
	}
	listener.SetUnlinkOnClose(true)
	if err := os.Chmod(host.Socket, 0600); err != nil {
		listener.Close()
		return nil, err
	}
	return accepting(listener), nil
}

func accepting(listener *net.UnixListener) *socket {
	ctx, cancel := context.WithCancel(context.Background())
	s := &socket{listener: listener, calls: make(chan call), cancel: cancel, finished: make(chan struct{})}
	go s.accept(ctx)
	return s
}

func (s *socket) accept(ctx context.Context) {
	defer close(s.finished)
	for {
		connection, err := s.listener.AcceptUnix()
		if err != nil {
			return
		}
		s.handle(ctx, connection)
		connection.Close()
	}
}

func (s *socket) handle(ctx context.Context, connection *net.UnixConn) {
	connection.SetDeadline(time.Now().Add(3 * time.Second))
	var value request
	decoder := json.NewDecoder(io.LimitReader(connection, 1024))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&value) != nil || decoder.Decode(new(any)) != io.EOF || !operation(value.Operation) {
		_ = write(connection, Diagnostic{Error: "expected one diagnostics or stop request"})
		return
	}
	connection.SetDeadline(time.Now().Add(15 * time.Second))
	c := call{operation: value.Operation, reply: make(chan Diagnostic, 1), finished: make(chan struct{})}
	defer close(c.finished)
	select {
	case s.calls <- c:
	case <-ctx.Done():
		return
	}
	select {
	case response := <-c.reply:
		_ = write(connection, response)
	case <-ctx.Done():
	}
}

func (s *socket) close() {
	s.cancel()
	s.listener.Close()
	<-s.finished
}

func operation(value string) bool { return value == "diagnostics" || value == "stop" }

func write(output io.Writer, value Diagnostic) error {
	if err := json.NewEncoder(output).Encode(value); err != nil {
		return fmt.Errorf("write client diagnostic: %w", err)
	}
	return nil
}
