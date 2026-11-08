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
	if command == "boot-protect" {
		return controller.ProtectBoot(ctx, output)
	}
	if command == "service-prepare" {
		return controller.PrepareService()
	}
	if command == "start" {
		return controller.Start(ctx, controller.Options{Profile: value.profile, Runtimes: value.runtimes, Network: value.network}, output)
	}
	if command == "service-run" {
		return controller.Managed(ctx, controller.Options{Profile: value.profile, Runtimes: value.runtimes, Network: value.network}, output)
	}
	if command == "resume" {
		return controller.Resume(ctx, output)
	}
	return controller.Command(ctx, command, output)
}
