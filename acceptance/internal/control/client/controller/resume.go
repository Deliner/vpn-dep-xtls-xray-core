package controller

import (
	"context"
	"fmt"
	"io"
	"time"

	"vpn-script/internal/control/client/host"
	"vpn-script/internal/control/client/session"
)

// Resume becomes the foreground controller of the last interrupted session.
// It neither creates a fresh journal nor releases its persistent firewall.
func Resume(ctx context.Context, output io.Writer) error {
	if err := host.LinuxRoot(); err != nil {
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
	directory, status, err := recorded(owner)
	if err != nil {
		return err
	}
	if directory == "" || status.Phase == "stopped" {
		return fmt.Errorf("no interrupted client session to resume")
	}
	value, err := loadRecorded(directory)
	if err != nil {
		return err
	}
	if err := validateIdentity(ctx, value); err != nil {
		return err
	}
	return resume(ctx, owner, value, status, output, false)
}

func resume(ctx context.Context, owner *host.Owner, value session.Request, status session.Status, output io.Writer, supervised bool) error {
	control, err := listen(owner)
	if err != nil {
		return err
	}
	defer control.close()
	limit, cancel := context.WithTimeout(ctx, 12*time.Second)
	active, err := session.Resume(limit, value, status)
	cancel()
	if active == nil {
		return err
	}
	return supervise(ctx, control, active, value, output, err, supervised)
}
