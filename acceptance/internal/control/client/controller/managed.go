package controller

import (
	"context"
	"fmt"
	"io"

	"vpn-script/internal/control/client/host"
	"vpn-script/internal/control/client/session"
	"vpn-script/internal/platform/safeio"
)

// PrepareService validates the controller state before systemd makes only that
// directory executable inside the service namespace. It never changes networking.
func PrepareService() error {
	if err := host.LinuxRoot(); err != nil {
		return err
	}
	owner, err := host.Acquire()
	if err != nil {
		return err
	}
	return owner.Close()
}

// Managed exits on engine failure for an external service supervisor to restart.
// It holds one controller lock across the decision to start or resume.
func Managed(ctx context.Context, options Options, output io.Writer) error {
	if err := host.LinuxRoot(); err != nil {
		return err
	}
	for _, path := range []string{options.Profile, options.Runtimes} {
		if err := host.PrivateInput(path); err != nil {
			return err
		}
	}
	desired, err := load(options)
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
	return managed(ctx, owner, desired, output)
}

func managed(ctx context.Context, owner *host.Owner, desired session.Request, output io.Writer) error {
	directory, status, err := recorded(owner)
	if err != nil {
		return err
	}
	if directory != "" && status.Phase != "stopped" {
		value, err := loadRecorded(directory)
		if err != nil {
			return err
		}
		if err := sameIntent(desired, value); err != nil {
			return err
		}
		if err := validateIdentity(ctx, value); err != nil {
			return err
		}
		return resume(ctx, owner, value, status, output, true)
	}
	value, err := prepare(ctx, owner, desired)
	if err != nil {
		return err
	}
	return serve(ctx, owner, value, output, true)
}

func sameIntent(desired, recorded session.Request) error {
	want, wantErr := safeio.Digest(desired.Profile)
	have, haveErr := safeio.Digest(recorded.Profile)
	desired.Network.EngineUID = recorded.Network.EngineUID
	if wantErr != nil || haveErr != nil || want != have || desired.Network != recorded.Network ||
		desired.Native.SHA256 != recorded.Native.SHA256 || desired.Frontend.SHA256 != recorded.Frontend.SHA256 || desired.Cronet.SHA256 != recorded.Cronet.SHA256 {
		return fmt.Errorf("service intent differs from protected session; explicit stop required before changing configuration")
	}
	return nil
}
