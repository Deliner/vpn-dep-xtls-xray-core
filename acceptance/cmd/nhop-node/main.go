package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"vpn-script/internal/core/model"
	"vpn-script/internal/node/installer"
	"vpn-script/internal/node/manifest"
	"vpn-script/internal/node/supervisor"
)

func run(ctx context.Context) error {
	supervise := flag.String("supervise", "", "run engines from an installed release")
	config := flag.String("config", "", "individual node JSON file")
	check := flag.Bool("check", false, "manifest validation only; no downloads or host changes")
	version := flag.Bool("version", false, "print installer version")
	flag.Parse()
	if *supervise != "" {
		err := supervisor.Run(ctx, *supervise)
		if errors.Is(err, context.Canceled) && ctx.Err() == context.Canceled {
			return nil
		}
		return err
	}
	if *version {
		fmt.Println("nhop-node 0.1.0")
		return nil
	}
	if *config == "" || flag.NArg() != 0 {
		return fmt.Errorf("use --config node.json [--check]")
	}
	data, err := manifest.Load(*config)
	if err != nil {
		return err
	}
	if *check {
		return json.NewEncoder(os.Stdout).Encode(model.Object{"node": data.NodeID, "generation": data.Generation,
			"listeners": data.Listeners, "validation": "manifest only; Xray and host checks run during installation"})
	}
	result, err := installer.New(ctx).Install(data)
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(result)
}

func main() {
	syscall.Umask(0077)
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	defer cancel()
	if err := run(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
