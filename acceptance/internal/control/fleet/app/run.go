// Package app binds Fleet controller and Linux agent operations to the CLI.
package app

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"

	"vpn-script/internal/node/fleet/service"
	"vpn-script/internal/platform/fleetapi"
)

const usage = "nhop-fleet init|serve|invite|assignment|publish|nodes|revoke|agent-init|agent-run [flags]\nnhop-fleet candidate|rollout-plan|rollout-create|rollouts|rollout|rollout-cancel [flags]\nnhop-fleet agent-service-install|agent-service-start|agent-service-stop|agent-service-remove [flags]\nUse COMMAND --help for its arguments. Agent commands require Linux root.\n"

func Run(ctx context.Context, args []string, output io.Writer) error {
	if len(args) == 0 || args[0] == "--help" {
		_, err := fmt.Fprint(output, usage)
		return err
	}
	if args[0] == "--supervise" {
		return supervise(ctx, args)
	}
	o, err := parse(args[0], args[1:], output)
	if errors.Is(err, flag.ErrHelp) {
		return nil
	}
	if err != nil {
		return err
	}
	switch args[0] {
	case "init":
		return initialize(o, output)
	case "serve":
		return serve(ctx, o)
	case "assignment":
		return assignment(o, output)
	case "agent-init", "agent-run", "agent-service-install", "agent-service-start", "agent-service-stop", "agent-service-remove":
		return agent(ctx, args[0], o, output)
	default:
		return admin(ctx, args[0], o, output)
	}
}

func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	var response *fleetapi.HTTPError
	if errors.As(err, &response) && (response.Status == http.StatusUnauthorized || response.Status == http.StatusForbidden) {
		return service.AccessDeniedExit
	}
	return 1
}
