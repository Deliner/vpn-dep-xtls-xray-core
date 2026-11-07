package controller

import (
	"context"
	"errors"
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
	return run(ctx, control.calls, active, initial)
}

func run(ctx context.Context, calls <-chan call, active *session.Session, state Diagnostic) error {
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
			if c.operation == "stop" {
				limit, cancel := context.WithTimeout(context.Background(), 12*time.Second)
				state = diagnostic(active, state.Directory, active.Stop(limit))
				cancel()
				done = nil
			}
			c.reply <- state
			if state.Status.Phase == "stopped" {
				<-c.finished // Deliver the stop acknowledgement before closing the socket.
				return nil
			}
		}
	}
}

func diagnostic(active *session.Session, directory string, err error) Diagnostic {
	value := Diagnostic{Controller: "active", Directory: directory, Status: active.Status()}
	if err != nil {
		value.Error = err.Error()
	}
	return value
}
