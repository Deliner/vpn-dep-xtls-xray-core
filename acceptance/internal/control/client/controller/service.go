package controller

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"vpn-script/internal/control/client/host"
	"vpn-script/internal/control/client/session"
)

// Start stays in the foreground. Only the explicit local stop command opens the
// network; SIGINT/SIGTERM cancellation retains protection for offline recovery.
func Start(ctx context.Context, options Options, output io.Writer) error {
	if err := host.LinuxRoot(); err != nil {
		return err
	}
	value, err := load(options)
	if err != nil {
		return err
	}
	if err := host.Commands(); err != nil {
		return err
	}
	owner, err := host.Acquire()
	if err != nil {
		return err
	}
	defer owner.Close()
	value, err = prepare(ctx, owner, value)
	if err != nil {
		return err
	}
	return serve(ctx, owner, value, output)
}

func serve(ctx context.Context, owner *host.Owner, value session.Request, output io.Writer) (result error) {
	control, err := listen(owner)
	if err != nil {
		return errors.Join(err, owner.ForgetUnstarted())
	}
	defer control.close()
	active, err := session.Start(ctx, value)
	if active == nil {
		return errors.Join(err, owner.ForgetUnstarted())
	}
	defer func() { result = errors.Join(result, active.Abort()) }()
	initial := diagnostic(active, value.Directory, err)
	if err := write(output, initial); err != nil {
		return err
	}
	return run(ctx, control.calls, active, initial, value)
}

func run(ctx context.Context, calls <-chan call, active *session.Session, state Diagnostic, request session.Request) error {
	var done <-chan struct{}
	if active.Status().Phase == "running" {
		done = active.Done()
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-done:
			state = diagnostic(active, state.Directory, active.Wait(ctx))
			done = nil
		case c := <-calls:
			response := controlSession(ctx, active, state, c.operation, request)
			if response.Status.Phase != "running" || response.Error == "" {
				state = response
			}
			done = nil
			if state.Status.Phase == "running" {
				done = active.Done()
			}
			c.reply <- response
			if state.Status.Phase == "stopped" {
				<-c.finished // Deliver the stop acknowledgement before closing the socket.
				return nil
			}
		}
	}
}

func controlSession(ctx context.Context, active *session.Session, state Diagnostic, operation string, request session.Request) Diagnostic {
	if operation == "diagnostics" {
		return state
	}
	if operation == "reconnect" {
		limit, cancel := context.WithTimeout(ctx, 12*time.Second)
		defer cancel()
		return diagnostic(active, state.Directory, reconnect(limit, active, request))
	}
	// Explicit stop keeps its independent cleanup budget after cancellation.
	cleanup, finish := context.WithTimeout(context.Background(), 12*time.Second)
	defer finish()
	return diagnostic(active, state.Directory, active.Stop(cleanup))
}

func reconnect(ctx context.Context, active *session.Session, request session.Request) error {
	identity, err := host.ExistingIdentity(ctx)
	if err != nil {
		return err
	}
	if identity.UID != request.Network.EngineUID || identity.GID != request.EngineGID {
		return fmt.Errorf("client NSS identity changed; reconnect refused with protection retained")
	}
	return active.Reconnect(ctx)
}

func diagnostic(active *session.Session, directory string, err error) Diagnostic {
	value := Diagnostic{Controller: "active", Directory: directory, Status: active.Status()}
	if err != nil {
		value.Error = err.Error()
	}
	return value
}
