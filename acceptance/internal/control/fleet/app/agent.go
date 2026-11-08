package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"

	"vpn-script/internal/core/fleet/wire"
	"vpn-script/internal/node/fleet/pull"
	"vpn-script/internal/node/fleet/service/install"
)

func agent(ctx context.Context, command string, o options, output io.Writer) error {
	if runtime.GOOS != "linux" || os.Geteuid() != 0 {
		return fmt.Errorf("Fleet agent commands require Linux root; no host state was changed")
	}
	if strings.HasPrefix(command, "agent-service-") {
		return agentService(ctx, command, o, output)
	}
	if err := protectedParent(o.state); err != nil {
		return err
	}
	if command == "agent-init" {
		var bootstrap pull.Bootstrap
		if err := privateRead(o.bootstrap, &bootstrap); err != nil {
			return err
		}
		return emit(output, map[string]string{"state": o.state}, pull.Initialize(o.state, bootstrap))
	}
	runner, err := pull.Open(o.state)
	if err != nil {
		return err
	}
	defer runner.Close()
	if o.once {
		status, err := runner.Step(ctx)
		return emit(output, status, err)
	}
	return observeAgent(ctx, runner, o, output)
}

func agentService(ctx context.Context, command string, o options, output io.Writer) error {
	if command == "agent-service-install" {
		receipt, err := install.Apply(ctx, install.Options{State: o.state, Interval: o.interval})
		return emit(output, receipt, err)
	}
	action := strings.TrimPrefix(command, "agent-service-")
	return emit(output, map[string]string{"agent_service": action}, install.Control(ctx, action))
}

func observeAgent(ctx context.Context, runner *pull.Runner, o options, output io.Writer) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var outputErr error
	err := runner.Run(ctx, o.interval, func(status wire.Status, failure error) {
		value := struct {
			Status wire.Status `json:"status"`
			OK     bool        `json:"ok"`
		}{Status: status, OK: failure == nil}
		outputErr = json.NewEncoder(output).Encode(value)
		if outputErr != nil {
			cancel()
		}
	})
	if errors.Is(err, context.Canceled) {
		err = nil
	}
	return errors.Join(err, outputErr)
}
