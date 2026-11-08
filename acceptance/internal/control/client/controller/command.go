package controller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"time"

	"vpn-script/internal/control/client/host"
	"vpn-script/internal/control/client/session"
)

func Command(ctx context.Context, operationName string, output io.Writer) error {
	if err := host.LinuxRoot(); err != nil {
		return err
	}
	if !operation(operationName) {
		return fmt.Errorf("unsupported client control command")
	}
	value, err := exchange(ctx, operationName)
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err != nil {
		value, err = offline(ctx, operationName)
	}
	if err != nil {
		return err
	}
	if err := write(output, value); err != nil {
		return err
	}
	if operationName != "diagnostics" && value.Error != "" {
		return errors.New(value.Error)
	}
	return nil
}

func exchange(ctx context.Context, operationName string) (Diagnostic, error) {
	if err := host.InspectSocket(); err != nil {
		return Diagnostic{}, err
	}
	dialer := net.Dialer{Timeout: 2 * time.Second}
	connection, err := dialer.DialContext(ctx, "unix", host.Socket)
	if err != nil {
		return Diagnostic{}, err
	}
	defer connection.Close()
	stop := context.AfterFunc(ctx, func() { connection.Close() })
	defer stop()
	connection.SetDeadline(time.Now().Add(20 * time.Second))
	if err := json.NewEncoder(connection).Encode(request{Operation: operationName}); err != nil {
		return Diagnostic{}, err
	}
	if err := connection.(*net.UnixConn).CloseWrite(); err != nil {
		return Diagnostic{}, err
	}
	var value Diagnostic
	decoder := json.NewDecoder(io.LimitReader(connection, 16384))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		return value, err
	}
	if decoder.Decode(new(any)) != io.EOF {
		return value, fmt.Errorf("invalid trailing control response")
	}
	return value, nil
}

func offline(ctx context.Context, operationName string) (Diagnostic, error) {
	if operationName == "reconnect" {
		return Diagnostic{}, fmt.Errorf("reconnect requires an active controller; protection retained")
	}
	owner, err := host.Acquire()
	if err != nil {
		return Diagnostic{}, err
	}
	defer owner.Close()
	directory, status, err := recorded(owner)
	value := Diagnostic{Controller: "unavailable", Directory: directory, Status: status}
	if err != nil || operationName != "stop" {
		return value, err
	}
	if directory == "" || status.Phase == "stopped" {
		value.Controller, value.Status.Phase = "inactive", "stopped"
		return value, nil
	}
	if err := host.Commands(); err != nil {
		return value, err
	}
	var recorded session.Request
	if err := host.ReadSession(directory, "request.json", &recorded); err != nil {
		return value, err
	}
	if recorded.Directory != directory {
		return value, fmt.Errorf("session request does not belong to current journal directory")
	}
	return recoverSession(ctx, recorded, status)
}

func recoverSession(ctx context.Context, recorded session.Request, status session.Status) (Diagnostic, error) {
	limit, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	active, err := session.Recover(limit, recorded, status)
	if err != nil {
		return Diagnostic{}, err
	}
	defer active.Abort()
	err = active.Stop(limit)
	value := diagnostic(active, recorded.Directory, err)
	value.Controller = "inactive"
	return value, nil
}
