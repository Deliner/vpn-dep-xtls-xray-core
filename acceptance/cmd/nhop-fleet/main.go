package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"vpn-script/internal/control/fleet/app"
)

func main() {
	syscall.Umask(0077)
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if err := app.Run(ctx, os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(app.ExitCode(err))
	}
}
