package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"

	"vpn-script/internal/control/client"
	"vpn-script/internal/control/deploy"
	"vpn-script/internal/control/probe"
	"vpn-script/internal/core/protocol"
	"vpn-script/internal/platform/process"
)

const usage = "nhop {validate|plan|render} topology.yaml [options]\nnhop deploy bundle [--jobs 1..64]\nnhop protocols\nnhop {verify bundle|client profile.json} --xray /path/to/xray --sing-box /path/to/sing-box\n"

func Run(ctx context.Context, args []string, output io.Writer) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "help" {
		_, err := fmt.Fprint(output, usage)
		return err
	}
	if args[0] == "version" {
		_, err := fmt.Fprintln(output, "nhop 0.1.0")
		return err
	}
	if args[0] == "protocols" {
		return writeJSON(output, protocol.All())
	}
	command := args[0]
	switch command {
	case "validate", "plan", "render", "deploy", "verify", "client":
	default:
		return fmt.Errorf("unknown command; run nhop --help")
	}
	options, err := parse(command, args[1:], output)
	if errors.Is(err, flag.ErrHelp) {
		return nil
	}
	if err != nil {
		return err
	}
	return execute(ctx, command, options, output)
}

func execute(ctx context.Context, command string, options options, output io.Writer) error {
	switch command {
	case "client":
		return client.Run(ctx, options.path, map[string]string{"xray": options.xray, "sing-box": options.singbox}, output)
	case "validate", "plan", "render":
		return topologyAction(command, options, output)
	case "deploy":
		return deploy.RunConcurrent(ctx, options.path, process.Run, output, options.jobs)
	default:
		report, err := probe.Verify(ctx, options.path, map[string]string{"xray": options.xray, "sing-box": options.singbox}, options.ipURL, process.Run)
		if printErr := writeJSON(output, report); printErr != nil {
			return errors.Join(err, printErr)
		}
		return err
	}
}
