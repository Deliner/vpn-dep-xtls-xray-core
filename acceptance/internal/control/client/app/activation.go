package app

import (
	"context"
	"io"
	"os"
	"os/signal"
	"syscall"

	"vpn-script/internal/control/client/controller"
)

func control(command string, value options, output io.Writer) error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if command == "start" {
		return controller.Start(ctx, controller.Options{Profile: value.profile, Runtimes: value.runtimes, Network: value.network}, output)
	}
	return controller.Command(ctx, command, output)
}
