package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"vpn-script/internal/quality/dependencies/archives"
	"vpn-script/internal/quality/dependencies/catalog"
	"vpn-script/internal/quality/dependencies/commands"
	"vpn-script/internal/quality/dependencies/modules"
	"vpn-script/internal/quality/dependencies/native"
	"vpn-script/internal/quality/dependencies/owned"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	result, err := execute(ctx, args)
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(result)
}

func execute(ctx context.Context, args []string) (any, error) {
	switch {
	case len(args) == 1 && args[0] == "inventory":
		return modules.Inventory(ctx, commands.Command)
	case len(args) == 3 && args[0] == "fork":
		return scopedFork(ctx, args[1], args[2])
	case len(args) == 1 && args[0] == "plan":
		return catalog.Build()
	case len(args) == 1 && args[0] == "resolved-plan":
		return catalog.BuildResolved()
	case len(args) == 1 && args[0] == "additional-plan":
		return catalog.BuildAdditional()
	case len(args) == 1 && args[0] == "enrich":
		return modules.Enrich()
	case len(args) == 3 && args[0] == "origins":
		return origins(ctx, args[1:])
	case len(args) == 3 && args[0] == "native":
		return native.Inventory(ctx, commands.Command, args[1], args[2])
	case len(args) == 3 && args[0] == "source":
		return archives.Fetch(ctx, commands.Command, args[1], args[2])
	case len(args) == 3 && args[0] == "owned-module":
		return owned.Download(ctx, commands.Command, args[1], args[2])
	default:
		return cleanupCommand(ctx, args)
	}
}
