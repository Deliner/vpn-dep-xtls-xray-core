package controller

import (
	"context"
	"io"

	"vpn-script/internal/control/client/guard/bootstrap"
)

// ProtectBoot installs offline egress protection before network configuration.
// Enabling it persistently is a separate, explicitly reviewed systemd operation.
func ProtectBoot(ctx context.Context, output io.Writer) error {
	if err := bootstrap.Activate(ctx); err != nil {
		return err
	}
	state, err := bootstrap.Inspect(ctx)
	if err != nil {
		return err
	}
	return write(output, Diagnostic{Controller: "inactive", Bootstrap: &state})
}
