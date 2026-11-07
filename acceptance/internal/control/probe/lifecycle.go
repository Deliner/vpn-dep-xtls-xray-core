package probe

import (
	"context"
	"fmt"
	"net"
	"os"
	"strconv"
	"time"

	"vpn-script/internal/platform/process"
	"vpn-script/internal/runtime/engines"
)

func launchClient(parent context.Context, binary, engine, path string, port int, log *os.File) (*probeClient, error) {
	ctx, cancel := context.WithCancelCause(parent)
	command := process.Command(ctx, binary, engines.Arguments(engine, path, false)...)
	command.Stdout, command.Stderr = log, log
	if err := command.Start(); err != nil {
		cancel(err)
		log.Close()
		return nil, err
	}
	local := &probeClient{port: port, ctx: ctx, cancel: cancel, command: command, done: make(chan struct{})}
	go func() {
		err := command.Wait()
		_ = process.Stop(command)
		_ = log.Close()
		if err == nil {
			err = fmt.Errorf("local runtime exited")
		} else {
			err = fmt.Errorf("local runtime exited: %w", err)
		}
		cancel(err)
		close(local.done)
	}()
	return local, nil
}

func (local *probeClient) stop() {
	local.cancel(context.Canceled)
	_ = process.Stop(local.command)
	<-local.done
}

func waitClient(local *probeClient) error {
	ctx, cancel := context.WithTimeout(local.ctx, 5*time.Second)
	defer cancel()
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	address := net.JoinHostPort("127.0.0.1", strconv.Itoa(local.port))
	dialer := net.Dialer{Timeout: 100 * time.Millisecond}
	for {
		if err := context.Cause(ctx); err != nil {
			return err
		}
		connection, err := dialer.DialContext(ctx, "tcp", address)
		if err == nil {
			connection.Close()
			return context.Cause(ctx)
		}
		select {
		case <-ctx.Done():
			return context.Cause(ctx)
		case <-ticker.C:
		}
	}
}
