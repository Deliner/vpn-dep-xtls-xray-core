package app

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"vpn-script/internal/control/client/systemd"
	"vpn-script/internal/control/client/systemd/install"
	"vpn-script/internal/platform/safeio"
)

func servicePlan(value options, output io.Writer) error {
	data, err := systemd.Unit(systemd.Options{Executable: value.binary, Profile: value.profile, Runtimes: value.runtimes, Network: value.network})
	if err != nil {
		return err
	}
	if err := safeio.Create(value.output, data, 0600); err != nil {
		return err
	}
	_, err = fmt.Fprintf(output, "Prepared systemd unit: %s; manual start only, no boot protection or installation performed\n", value.output)
	return err
}

func serviceInstall(command string, value options, output io.Writer) error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if command == "service-remove" {
		if err := install.Remove(ctx); err != nil {
			return err
		}
		_, err := fmt.Fprintln(output, "Removed manual-start service; snapshots and recovery state retained; networking unchanged")
		return err
	}
	r, err := install.Apply(ctx, install.Options{Profile: value.profile, Runtimes: value.runtimes, Network: value.network})
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(output, "Installed generation %s at %s\nUnit: %s; inactive, manual start only; no boot protection\n", r.Generation, r.Directory, r.Unit)
	return err
}
